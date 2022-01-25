package datastore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/gogo/protobuf/jsonpb"
	ptypes "github.com/gogo/protobuf/types"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ByCreatedAt []*api.Job

func (s ByCreatedAt) Len() int           { return len(s) }
func (s ByCreatedAt) Less(i, j int) bool { return s[i].CreatedAt.After(s[j].CreatedAt) }
func (s ByCreatedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// GetJobs returns a list of jobs
func (d *Datastore) GetJobs(ctx context.Context) ([]*api.Job, error) {
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    finca.S3ProjectPath,
		Recursive: true,
	})

	found := map[string]struct{}{}
	jobs := []*api.Job{}
	for o := range objCh {
		if o.Err != nil {
			return nil, errors.Wrap(o.Err, "error reading jobs from datastore")
		}

		k := strings.SplitN(o.Key, "/", 2)
		jobID := path.Dir(k[1])

		if _, ok := found[jobID]; ok {
			continue
		}

		found[jobID] = struct{}{}

		job, err := d.GetJob(ctx, jobID)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting job from datastore: %s", jobID)
		}

		jobs = append(jobs, job)
	}
	sort.Sort(ByCreatedAt(jobs))

	return jobs, nil
}

// GetJob returns a job by id from the datastore
func (d *Datastore) GetJob(ctx context.Context, jobID string) (*api.Job, error) {
	jobPath := path.Join(finca.S3ProjectPath, jobID, finca.S3JobPath)
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("key not found: %s", jobPath)
		return nil, err
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	job := &api.Job{}
	if err := jsonpb.Unmarshal(buf, job); err != nil {
		return nil, err
	}

	// check for render slices
	if job.Request.RenderSlices > 0 {
		for i := int64(0); i < job.Request.RenderSlices; i++ {
			jobPath := path.Join(finca.S3ProjectPath, jobID, fmt.Sprintf("%s.%d", finca.S3JobPath, i))
			obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
			if err != nil {
				return nil, err
			}

			j := &api.Job{}
			buf := &bytes.Buffer{}
			if _, err := io.Copy(buf, obj); err != nil {
				// if job output doesn't exist it is most likely queued and hasn't started yet
				if strings.Contains(err.Error(), "key does not exist") {
					logrus.Debugf("key not found for %s; assuming slice job is queued", jobPath)
					job.SliceJobs = append(job.SliceJobs, j)
					continue
				}
				return nil, err
			}

			if err := jsonpb.Unmarshal(buf, j); err != nil {
				return nil, err
			}

			job.SliceJobs = append(job.SliceJobs, j)
		}

		// "merge" slice jobs
		if err := mergeSliceJobs(job); err != nil {
			return nil, err
		}
	}

	return job, nil
}

// UpdateJob updates the status of a job
func (d *Datastore) UpdateJob(ctx context.Context, job *api.Job) error {
	jobPath := getJobPath(job.ID, int(job.RenderSliceIndex))

	j, err := d.GetJob(ctx, job.ID)
	if err != nil {
		if !strings.Contains(err.Error(), "key does not exist") {
			return err
		}
	}

	// update from existing if zero
	if j != nil {
		if job.QueuedAt.IsZero() {
			job.QueuedAt = j.QueuedAt
		}
		if job.StartedAt.IsZero() {
			job.StartedAt = j.StartedAt
		}
		if job.FinishedAt.IsZero() {
			job.FinishedAt = j.FinishedAt
		}

	}

	buf := &bytes.Buffer{}
	if err := d.Marshaler().Marshal(buf, job); err != nil {
		return err
	}

	if _, err := d.storageClient.PutObject(ctx, d.config.S3Bucket, jobPath, buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: finca.S3JobContentType}); err != nil {
		return errors.Wrap(err, "error updating job status")
	}

	return nil
}

// DeleteJob deletes a job and all related files from the datastore
func (d *Datastore) DeleteJob(ctx context.Context, id string) error {
	// delete project
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(finca.S3ProjectPath, id),
		Recursive: true,
	})

	for o := range objCh {
		if o.Err != nil {
			return o.Err
		}

		if err := d.storageClient.RemoveObject(ctx, d.config.S3Bucket, o.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	// delete renders
	rObjCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(finca.S3RenderPath, id),
		Recursive: true,
	})

	for o := range rObjCh {
		if o.Err != nil {
			return o.Err
		}

		if err := d.storageClient.RemoveObject(ctx, d.config.S3Bucket, o.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func getStoragePath(jobID string) string {
	return path.Join(finca.S3ProjectPath, jobID)
}

func getJobPath(jobID string, renderSliceIndex int) string {
	jobPath := path.Join(getStoragePath(jobID), finca.S3JobPath)
	// check for render slice
	if idx := renderSliceIndex; idx > -1 {
		jobPath = path.Join(jobPath) + fmt.Sprintf(".%d", idx)
	}
	return jobPath
}

// mergeSliceJobs "merges" and "resolves" all slice jobs in the passed job
// it will set the status at the least progressed and set the
// render time at the longest time of the individual slices
func mergeSliceJobs(job *api.Job) error {
	renderTimeSeconds := float64(0.0)
	job.Status = api.Job_QUEUED
	finishedJobs := 0
	for i := 0; i < len(job.SliceJobs); i++ {
		sliceJob := job.SliceJobs[i]
		// check status based upon slice
		sliceStatusV := api.Job_Status_value[sliceJob.Status.String()]
		jobStatusV := api.Job_Status_value[job.Status.String()]
		if sliceJob.Status == api.Job_FINISHED {
			finishedJobs += 1
		}
		if sliceStatusV > jobStatusV {
			switch job.Status {
			case api.Job_RENDERING, api.Job_ERROR:
				// leave current status
			case api.Job_QUEUED, api.Job_FINISHED:
				// only update if not finished
				switch sliceJob.Status {
				case api.Job_FINISHED:
					job.Status = api.Job_RENDERING
				default:
					job.Status = sliceJob.Status
				}
			}
		}
		if sliceStatusV < jobStatusV {
			switch sliceJob.Status {
			case api.Job_RENDERING, api.Job_ERROR:
				job.Status = sliceJob.Status
			case api.Job_QUEUED:
				switch job.Status {
				case api.Job_RENDERING, api.Job_ERROR:
					// leave
				default:
					job.Status = sliceJob.Status
				}
			}
		}

		// set time to default if zero
		if job.StartedAt.IsZero() {
			job.StartedAt = sliceJob.StartedAt
		}
		// update times
		if sliceJob.StartedAt.Before(job.StartedAt) {
			job.StartedAt = sliceJob.StartedAt
		}

		if sliceJob.FinishedAt.After(job.FinishedAt) {
			job.FinishedAt = sliceJob.FinishedAt
		}
		if sliceJob.Duration != nil {
			renderTimeSeconds += float64(sliceJob.Duration.Seconds)
		}
	}
	// check for finished jobs to mark as finished
	if finishedJobs == len(job.SliceJobs) {
		job.Status = api.Job_FINISHED
	}

	renderTime, err := time.ParseDuration(fmt.Sprintf("%0.2fs", renderTimeSeconds))
	if err != nil {
		return err
	}
	job.Duration = ptypes.DurationProto(renderTime)

	return nil
}
