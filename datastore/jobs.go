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
	logrus.Debugf("getting job from path: %s", jobPath)
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
	if err != nil {
		logrus.Errorf("key not found: %s", jobPath)
		return nil, err
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	j := &api.Job{}
	if err := jsonpb.Unmarshal(buf, j); err != nil {
		return nil, err
	}

	// check for render slices
	if j.Request.RenderSlices > 0 {
		sliceJobs := []*api.Job{}
		for i := int64(0); i < j.Request.RenderSlices; i++ {
			jobPath := path.Join(finca.S3ProjectPath, jobID, fmt.Sprintf("%s.%d", finca.S3JobPath, i))
			logrus.Debugf("loading job slice from %s", jobPath)
			obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
			if err != nil {
				return nil, err
			}

			buf := &bytes.Buffer{}
			if _, err := io.Copy(buf, obj); err != nil {
				// if job output doesn't exist it is most likely queued and hasn't started yet
				if strings.Contains(err.Error(), "key does not exist") {
					logrus.Debugf("key not found for %s; assuming slice job is queued", jobPath)
					sliceJobs = append(sliceJobs, j)
					continue
				}
				return nil, err
			}

			j := &api.Job{}
			if err := jsonpb.Unmarshal(buf, j); err != nil {
				return nil, err
			}

			sliceJobs = append(sliceJobs, j)
		}

		// "merge" slice jobs
		mergedJob, err := mergeSliceJobs(sliceJobs)
		if err != nil {
			return nil, err
		}
		j = mergedJob
	}

	return j, nil
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
	m := &jsonpb.Marshaler{}
	if err := m.Marshal(buf, job); err != nil {
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

// mergeSliceJobs "merges" all slice jobs into a single job
// it will set the status at the least progressed and set the
// render time at the longest time of the individual slices
func mergeSliceJobs(sliceJobs []*api.Job) (*api.Job, error) {
	// start with base job
	job := sliceJobs[0]
	job.SliceSequenceIDs = []uint64{}

	renderTimeSeconds := float64(0.0)

	for _, j := range sliceJobs {
		logrus.Debugf("merging slice job %s", j.ID)
		if api.Job_Status_value[j.Status.String()] < api.Job_Status_value[job.Status.String()] {
			job.Status = j.Status
		}
		if j.StartedAt.Before(job.StartedAt) {
			job.StartedAt = job.StartedAt
		}

		if j.FinishedAt.After(job.FinishedAt) {
			job.FinishedAt = j.FinishedAt
		}
		// set slice sequence ids
		logrus.Debugf("appending slice sequence id %d", j.SequenceID)
		job.SliceSequenceIDs = append(job.SliceSequenceIDs, j.SequenceID)
		if job.Duration != nil {
			renderTimeSeconds += float64(job.Duration.Seconds)
		}
	}

	renderTime, err := time.ParseDuration(fmt.Sprintf("%0.2fs", renderTimeSeconds))
	if err != nil {
		return nil, err
	}
	job.Duration = ptypes.DurationProto(renderTime)

	return job, nil
}
