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

type JobOptConfig struct {
	excludeFrames bool
	excludeSlices bool
}

type JobOpt func(c *JobOptConfig)

// GetJobs returns a list of jobs
func (d *Datastore) GetJobs(ctx context.Context, jobOpts ...JobOpt) ([]*api.Job, error) {
	cfg := &JobOptConfig{}
	for _, o := range jobOpts {
		o(cfg)
	}

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

		job, err := d.GetJob(ctx, jobID, jobOpts...)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting job from datastore: %s", jobID)
		}

		jobs = append(jobs, job)
	}
	sort.Sort(ByCreatedAt(jobs))

	return jobs, nil
}

// GetJob returns a job by id from the datastore
func (d *Datastore) GetJob(ctx context.Context, jobID string, jobOpts ...JobOpt) (*api.Job, error) {
	cfg := &JobOptConfig{}
	for _, o := range jobOpts {
		o(cfg)
	}

	jobPath := getJobPath(jobID)
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "error getting job %s", jobPath)
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	job := &api.Job{}
	if err := jsonpb.Unmarshal(buf, job); err != nil {
		return nil, err
	}
	// filter
	if cfg.excludeSlices {
		for _, j := range job.FrameJobs {
			j.SliceJobs = []*api.SliceJob{}
		}
	}
	if cfg.excludeFrames {
		job.FrameJobs = []*api.FrameJob{}
	}

	return job, nil
}

// ResolveJob resolves a job from all frame and slice jobs
func (d *Datastore) ResolveJob(ctx context.Context, jobID string) error {
	jobPath := getJobPath(jobID)
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
	if err != nil {
		return errors.Wrapf(err, "error getting job %s", jobPath)
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return err
	}

	job := &api.Job{}
	if err := jsonpb.Unmarshal(buf, job); err != nil {
		return err
	}

	frameJobs := []*api.FrameJob{}

	// load frames
	for _, f := range job.FrameJobs {
		// check if non-slice job
		if job.Request.RenderSlices == 0 {
			framePath := getFrameJobPath(jobID, f.RenderFrame)
			obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, framePath, minio.GetObjectOptions{})
			if err != nil {
				if strings.Contains(err.Error(), "key does not exist") {
					logrus.Debugf("key not found for %s; assuming frame job is queued", jobPath)
					continue
				}
				return errors.Wrapf(err, "error getting frame job object %s", framePath)
			}

			buf := &bytes.Buffer{}
			if _, err := io.Copy(buf, obj); err != nil {
				return err
			}

			frameJob := &api.FrameJob{}
			if err := jsonpb.Unmarshal(buf, frameJob); err != nil {
				return err
			}
			frameJobs = append(frameJobs, frameJob)
			continue
		}
		// load slices
		sliceJobs := []*api.SliceJob{}
		for _, slice := range f.SliceJobs {
			slicePath := getSliceJobPath(jobID, slice.RenderFrame, slice.RenderSliceIndex)
			obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, slicePath, minio.GetObjectOptions{})
			if err != nil {
				if strings.Contains(err.Error(), "key does not exist") {
					logrus.Debugf("key not found for %s; assuming slice job is queued", jobPath)
					continue
				}
				return err
			}
			sliceJob := &api.SliceJob{}
			buf := &bytes.Buffer{}
			if _, err := io.Copy(buf, obj); err != nil {
				return err
			}

			if err := jsonpb.Unmarshal(buf, sliceJob); err != nil {
				return err
			}

			sliceJobs = append(sliceJobs, sliceJob)
		}
		f.SliceJobs = sliceJobs
		frameJobs = append(frameJobs, f)
	}

	job.FrameJobs = frameJobs

	if err := resolveJob(job); err != nil {
		return err
	}

	if err := d.UpdateJob(ctx, jobID, job); err != nil {
		return err
	}

	return nil
}

// UpdateJob updates the status of a job
func (d *Datastore) UpdateJob(ctx context.Context, jobID string, o interface{}) error {
	dbPath := ""
	buf := &bytes.Buffer{}

	resolveJob := false

	switch v := o.(type) {
	case *api.Job:
		dbPath = getJobPath(jobID)
		if err := d.Marshaler().Marshal(buf, v); err != nil {
			return err
		}
	case *api.FrameJob:
		dbPath = getFrameJobPath(jobID, v.RenderFrame)
		if err := d.Marshaler().Marshal(buf, v); err != nil {
			return err
		}
		resolveJob = true
	case *api.SliceJob:
		dbPath = getSliceJobPath(jobID, v.RenderFrame, v.RenderSliceIndex)
		if err := d.Marshaler().Marshal(buf, v); err != nil {
			return err
		}
		resolveJob = true
	default:
		return fmt.Errorf("job type %+T is unknown", v)
	}

	if _, err := d.storageClient.PutObject(ctx, d.config.S3Bucket, dbPath, buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: finca.S3JobContentType}); err != nil {
		return errors.Wrap(err, "error updating job status")
	}

	if resolveJob {
		logrus.Debugf("resolving job %s", jobID)
		if err := d.ResolveJob(ctx, jobID); err != nil {
			return errors.Wrapf(err, "error resolving job %s", jobID)
		}
	}

	return nil
}

// DeleteJob deletes a job and all related files from the datastore
func (d *Datastore) DeleteJob(ctx context.Context, id string) error {
	// TODO: can just the top level keys be deleted and recurse instead of iterating objects?

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

func getJobPath(jobID string) string {
	jobPath := path.Join(getStoragePath(jobID), finca.S3JobPath)
	return jobPath
}

func getFrameJobPath(jobID string, frame int64) string {
	return path.Join(getStoragePath(jobID), fmt.Sprintf("frame_%04d_%s", frame, finca.S3JobPath))
}

func getSliceJobPath(jobID string, frame int64, renderSliceIndex int64) string {
	return fmt.Sprintf("%s.%d", getFrameJobPath(jobID, frame), renderSliceIndex)
}

// WARNING: dragons ahead....
//
// resolveJob "merges" and "resolves" all frame and slice jobs
// in the passed job and will set the status at the least progressed and set the
// render time at the longest time of the individual frames and slices
func resolveJob(job *api.Job) error {
	renderTimeSeconds := float64(0.0)
	job.Status = api.JobStatus_QUEUED
	finishedFrames := 0
	for _, frameJob := range job.FrameJobs {
		finishedSlices := 0
		frameRenderSeconds := float64(0.0)
		for _, sliceJob := range frameJob.SliceJobs {
			// check status based upon slice
			sliceStatusV := api.JobStatus_value[sliceJob.Status.String()]
			jobStatusV := api.JobStatus_value[job.Status.String()]
			if sliceJob.Status == api.JobStatus_FINISHED {
				finishedSlices += 1
			}
			if sliceStatusV > jobStatusV {
				switch job.Status {
				case api.JobStatus_RENDERING, api.JobStatus_ERROR:
					// leave current status
				case api.JobStatus_QUEUED, api.JobStatus_FINISHED:
					// only update if not finished
					switch sliceJob.Status {
					case api.JobStatus_FINISHED:
						job.Status = api.JobStatus_RENDERING
						frameJob.Status = api.JobStatus_RENDERING
					default:
						job.Status = sliceJob.Status
						frameJob.Status = sliceJob.Status
					}
				}
			}
			if sliceStatusV < jobStatusV {
				switch sliceJob.Status {
				case api.JobStatus_RENDERING, api.JobStatus_ERROR:
					job.Status = sliceJob.Status
				case api.JobStatus_QUEUED:
					switch job.Status {
					case api.JobStatus_RENDERING, api.JobStatus_ERROR:
						// leave
					default:
						job.Status = sliceJob.Status
						frameJob.Status = sliceJob.Status
					}
				}
			}

			// update frame times
			// set time to default if zero
			if frameJob.StartedAt.IsZero() {
				frameJob.StartedAt = sliceJob.StartedAt
			}
			// update times
			if sliceJob.StartedAt.Before(frameJob.StartedAt) {
				frameJob.StartedAt = sliceJob.StartedAt
			}

			if sliceJob.FinishedAt.After(frameJob.FinishedAt) {
				frameJob.FinishedAt = sliceJob.FinishedAt
			}

			// update job times
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
			// calculate slice duration
			if !sliceJob.StartedAt.IsZero() {
				if !sliceJob.FinishedAt.IsZero() {
					frameRenderSeconds += float64(sliceJob.FinishedAt.Sub(sliceJob.StartedAt).Seconds())
				}
			}
		}

		// slices
		if job.Request.RenderSlices > 0 && finishedSlices == len(frameJob.SliceJobs) {
			//finishedFrames += 1
			frameJob.Status = api.JobStatus_FINISHED
			renderTimeSeconds += frameRenderSeconds
		}

		// frame status
		frameStatusV := api.JobStatus_value[frameJob.Status.String()]
		jobStatusV := api.JobStatus_value[job.Status.String()]
		if frameJob.Status == api.JobStatus_FINISHED {
			finishedFrames += 1
		}
		if frameStatusV > jobStatusV {
			switch job.Status {
			case api.JobStatus_RENDERING, api.JobStatus_ERROR:
				// leave current status
			case api.JobStatus_QUEUED, api.JobStatus_FINISHED:
				// only update if not finished
				switch frameJob.Status {
				case api.JobStatus_FINISHED:
					job.Status = api.JobStatus_RENDERING
					//frameJob.Status = api.JobStatus_RENDERING
				default:
					job.Status = frameJob.Status
				}
			}
		}
		if frameStatusV < jobStatusV {
			switch frameJob.Status {
			case api.JobStatus_RENDERING, api.JobStatus_ERROR:
				job.Status = frameJob.Status
			case api.JobStatus_QUEUED:
				switch job.Status {
				case api.JobStatus_RENDERING, api.JobStatus_ERROR:
					// leave
				default:
					job.Status = frameJob.Status
				}
			}
		}
		// update job times
		// set time to default if zero
		if job.StartedAt.IsZero() {
			job.StartedAt = frameJob.StartedAt
		}
		// update times
		if frameJob.StartedAt.Before(job.StartedAt) {
			job.StartedAt = frameJob.StartedAt
		}

		if frameJob.FinishedAt.After(job.FinishedAt) {
			job.FinishedAt = frameJob.FinishedAt
		}

		// calculate frame duration if not using slices
		if job.Request.RenderSlices == 0 && !frameJob.StartedAt.IsZero() {
			if !frameJob.FinishedAt.IsZero() {
				renderTimeSeconds += float64(frameJob.FinishedAt.Sub(frameJob.StartedAt).Seconds())
			}
		}
	}

	// check for finished jobs to mark as finished
	if finishedFrames == len(job.FrameJobs) {
		job.Status = api.JobStatus_FINISHED
	}

	renderTime, err := time.ParseDuration(fmt.Sprintf("%0.2fs", renderTimeSeconds))
	if err != nil {
		return err
	}
	job.Duration = ptypes.DurationProto(renderTime)

	return nil
}
