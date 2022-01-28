package datastore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
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

	keys, err := d.redisClient.Keys(ctx, getJobKey("*")).Result()
	if err != nil {
		return nil, err
	}

	jobs := []*api.Job{}
	for _, k := range keys {
		parts := strings.Split(k, "/")
		if len(parts) != 3 {
			logrus.Errorf("invalid key format: %s", k)
			continue
		}
		jobID := parts[1]
		job, err := d.GetJob(ctx, jobID)
		if err != nil {
			return nil, err
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

	jobKey := getJobKey(jobID)
	data, err := d.redisClient.Get(ctx, jobKey).Bytes()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting job %s from database", jobID)
	}

	buf := bytes.NewBuffer(data)
	job := &api.Job{}
	if err := jsonpb.Unmarshal(buf, job); err != nil {
		return nil, err
	}

	job.FrameJobs = []*api.FrameJob{}

	// filter
	if !cfg.excludeFrames {
		for frame := job.Request.RenderStartFrame; frame <= job.Request.RenderEndFrame; frame++ {
			frameKey := getFrameKey(jobID, frame)
			data, err := d.redisClient.Get(ctx, frameKey).Bytes()
			if err != nil {
				return nil, errors.Wrapf(err, "error getting frame %d for job %s from database", frame, jobID)
			}

			buf := bytes.NewBuffer(data)
			frameJob := &api.FrameJob{}
			if err := jsonpb.Unmarshal(buf, frameJob); err != nil {
				return nil, err
			}
			if !cfg.excludeSlices {
				if job.Request.RenderSlices > 0 {
					frameJob.SliceJobs = []*api.SliceJob{}
					for i := int64(0); i < job.Request.RenderSlices; i++ {
						sliceKey := getSliceKey(jobID, frame, i)
						data, err := d.redisClient.Get(ctx, sliceKey).Bytes()
						if err != nil {
							return nil, errors.Wrapf(err, "error getting slice %d (frame %d) for job %s from database", i, frame, jobID)
						}

						buf := bytes.NewBuffer(data)
						sliceJob := &api.SliceJob{}
						if err := jsonpb.Unmarshal(buf, sliceJob); err != nil {
							return nil, err
						}
						frameJob.SliceJobs = append(frameJob.SliceJobs, sliceJob)
					}
				}
			}
			job.FrameJobs = append(job.FrameJobs, frameJob)
		}
	}

	if err := resolveJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (d *Datastore) GetJobLog(ctx context.Context, jobID string) (*api.JobLog, error) {
	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	switch job.Status {
	case api.JobStatus_QUEUED, api.JobStatus_RENDERING:
		return nil, nil
	}

	logPath := getJobLogPath(jobID)
	// get key data
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, logPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "error getting log from storage %s", logPath)
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	return &api.JobLog{
		Log: string(buf.Bytes()),
	}, nil
}

func (d *Datastore) GetRenderLog(ctx context.Context, jobID string, frame int64, slice int64) (*api.RenderLog, error) {
	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	switch job.Status {
	case api.JobStatus_QUEUED, api.JobStatus_RENDERING:
		return nil, nil
	}

	renderPath := path.Join(finca.S3RenderPath, jobID)
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    renderPath,
		Recursive: true,
	})

	logPath := ""
	for o := range objCh {
		if o.Err != nil {
			return nil, o.Err
		}

		// filter logs
		if filepath.Ext(o.Key) != ".log" {
			continue
		}
		// ignore slice renders
		if slice > -1 {
			sliceMatch, err := regexp.MatchString(fmt.Sprintf(".*_slice_%d_%04d.log", slice, frame), o.Key)
			if err != nil {
				return nil, err
			}
			if sliceMatch {
				logPath = o.Key
				break
			}
		}
		frameMatch, err := regexp.MatchString(fmt.Sprintf(".*_%04d.log", frame), o.Key)
		if err != nil {
			return nil, err
		}
		if frameMatch {
			logPath = o.Key
			break
		}
	}

	if logPath == "" {
		return nil, nil
	}

	// get key data
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, logPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "error getting log from storage %s", logPath)
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	return &api.RenderLog{
		Log: string(buf.Bytes()),
	}, nil
}

// ResolveJob resolves a job from all frame and slice jobs and stores the result in the database
func (d *Datastore) ResolveJob(ctx context.Context, jobID string) error {
	jobKey := getJobKey(jobID)
	data, err := d.redisClient.Get(ctx, jobKey).Bytes()
	if err != nil {
		return errors.Wrapf(err, "error getting job %s from database to resolve", jobID)
	}

	buf := bytes.NewBuffer(data)
	job := &api.Job{}
	if err := jsonpb.Unmarshal(buf, job); err != nil {
		return err
	}

	keys, err := d.redisClient.Keys(ctx, getJobKey("*")).Result()
	if err != nil {
		return err
	}

	jobs := []*api.Job{}
	for _, k := range keys {
		parts := strings.Split(k, "/")
		if len(parts) != 3 {
			logrus.Errorf("invalid key format: %s", k)
			continue
		}
		jobID := parts[1]
		job, err := d.GetJob(ctx, jobID)
		if err != nil {
			return err
		}
		jobs = append(jobs, job)
	}
	frameJobs := []*api.FrameJob{}

	// load frames
	for _, f := range job.FrameJobs {
		// check if non-slice job
		if job.Request.RenderSlices == 0 {
			frameKey := getFrameKey(jobID, f.RenderFrame)
			data, err := d.redisClient.Get(ctx, frameKey).Bytes()
			if err != nil {
				return errors.Wrapf(err, "error getting frame %d for job %s from database", f.RenderFrame, jobID)
			}

			buf := bytes.NewBuffer(data)
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
			sliceKey := getSliceKey(jobID, slice.RenderFrame, slice.RenderSliceIndex)
			data, err := d.redisClient.Get(ctx, sliceKey).Bytes()
			if err != nil {
				return errors.Wrapf(err, "error getting slice %d (frame %d) for job %s from database", slice.RenderSliceIndex, f.RenderFrame, jobID)
			}

			buf := bytes.NewBuffer(data)
			sliceJob := &api.SliceJob{}
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
	dbKey := ""
	buf := &bytes.Buffer{}

	switch v := o.(type) {
	case *api.Job:
		dbKey = getJobKey(jobID)
		if err := d.Marshaler().Marshal(buf, v); err != nil {
			return err
		}
	case *api.FrameJob:
		dbKey = getFrameKey(jobID, v.RenderFrame)
		if err := d.Marshaler().Marshal(buf, v); err != nil {
			return err
		}
	case *api.SliceJob:
		dbKey = getSliceKey(jobID, v.RenderFrame, v.RenderSliceIndex)
		if err := d.Marshaler().Marshal(buf, v); err != nil {
			return err
		}
	default:
		return fmt.Errorf("job type %+T is unknown", v)
	}

	if err := d.redisClient.Set(ctx, dbKey, buf.Bytes(), 0).Err(); err != nil {
		return errors.Wrapf(err, "error saving job %s in database", jobID)
	}

	return nil
}

// DeleteJob deletes a job and all related files from the datastore
func (d *Datastore) DeleteJob(ctx context.Context, jobID string) error {
	// delete from db
	keys := []string{
		getJobKey(jobID),
	}

	frameKeys, err := d.redisClient.Keys(ctx, getFrameKey(jobID, "*")).Result()
	if err != nil {
		return err
	}

	keys = append(keys, frameKeys...)

	sliceKeys, err := d.redisClient.Keys(ctx, getSliceKey(jobID, "*", "*")).Result()
	if err != nil {
		return err
	}
	keys = append(keys, sliceKeys...)

	if err := d.redisClient.Del(ctx, keys...).Err(); err != nil {
		return err
	}

	// delete project
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(finca.S3ProjectPath, jobID),
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
		Prefix:    path.Join(finca.S3RenderPath, jobID),
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

func getJobLogPath(jobID string) string {
	return path.Join(getStoragePath(jobID), finca.S3JobLogPath)
}

func getFrameJobPath(jobID string, frame int64) string {
	return path.Join(getStoragePath(jobID), fmt.Sprintf("frame_%04d_%s", frame, finca.S3JobPath))
}

func getSliceJobPath(jobID string, frame int64, renderSliceIndex int64) string {
	return fmt.Sprintf("%s.%d", getFrameJobPath(jobID, frame), renderSliceIndex)
}

func getJobKey(jobID string) string {
	return path.Join(dbPrefix, jobID, "job")
}

func getFrameKey(jobID string, frame interface{}) string {
	return path.Join(dbPrefix, "frames", jobID, fmt.Sprintf("%v", frame))
}

func getSliceKey(jobID string, frame interface{}, sliceIndex interface{}) string {
	return path.Join(dbPrefix, "slices", jobID, fmt.Sprintf("%v", frame), fmt.Sprintf("%v", sliceIndex))
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
