// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/jobs/v1"
	"git.underland.io/ehazlett/fynca/pkg/tracing"
	"github.com/go-redis/redis/v8"
	"github.com/gogo/protobuf/proto"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	jobCacheTTL = time.Second * 10
)

type ByCreatedAt []*api.Job

func (s ByCreatedAt) Len() int           { return len(s) }
func (s ByCreatedAt) Less(i, j int) bool { return s[i].CreatedAt.After(s[j].CreatedAt) }
func (s ByCreatedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// GetJobs returns a list of jobs
func (d *Datastore) GetJobs(ctx context.Context) ([]*api.Job, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetJobs")
	defer span.End()

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	span.SetAttributes(attribute.String("namespace", namespace))

	keys, err := d.redisClient.Keys(ctx, getJobKey(namespace, "*")).Result()
	if err != nil {
		return nil, err
	}

	jobs := []*api.Job{}
	for _, k := range keys {
		parts := strings.Split(k, "/")
		if len(parts) != 5 {
			logrus.Errorf("invalid key format: %s", k)
			continue
		}
		jobID := parts[3]
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
func (d *Datastore) GetJob(ctx context.Context, jobID string) (*api.Job, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetJob")
	defer span.End()

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("jobID", jobID))

	// lookup from cache
	cacheKey := getJobCacheKey(jobID)
	// TODO: revisit caching
	//// cache url and retrieve if less than the ttl
	//cacheData, err := d.GetCacheObject(ctx, cacheKey)
	//if err != nil {
	//	return nil, err
	//}
	//if cacheData != nil {
	//	logrus.Debugf("fetching job %s from cache", jobID)
	//	_, cspan := d.tracer().Start(ctx, "GetJob.GetCacheObject")
	//	cspan.SetAttributes(attribute.String("cacheKey", cacheKey))
	//	var j api.Job
	//	if err := proto.Unmarshal(cacheData, &j); err != nil {
	//		logrus.WithError(err).Errorf("error unmarshaling job %s from cache", jobID)
	//	}
	//	cspan.End()

	//	return &j, nil
	//}

	jobKey := getJobKey(namespace, jobID)

	data, err := d.redisClient.Get(ctx, jobKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, status.Errorf(codes.NotFound, "job %s not found", jobID)
		}
		span.RecordError(err)
		return nil, errors.Wrapf(err, "error getting job %s from database", jobID)
	}

	job := api.Job{}
	if err := proto.Unmarshal(data, &job); err != nil {
		span.RecordError(err)
		return nil, err
	}

	job.FrameJobs = []*api.FrameJob{}
	span.SetAttributes(attribute.Int64("renderStartFrame", job.Request.RenderStartFrame))
	span.SetAttributes(attribute.Int64("renderEndFrame", job.Request.RenderEndFrame))

	// filter
	for frame := job.Request.RenderStartFrame; frame <= job.Request.RenderEndFrame; frame++ {
		frameKey := getFrameKey(namespace, jobID, frame)
		data, err := d.redisClient.Get(ctx, frameKey).Bytes()
		if err != nil {
			return nil, errors.Wrapf(err, "error getting frame %d for job %s from database", frame, jobID)
		}

		frameJob := api.FrameJob{}
		if err := proto.Unmarshal(data, &frameJob); err != nil {
			return nil, err
		}

		if job.Request.RenderSlices > 0 {
			frameJob.SliceJobs = []*api.SliceJob{}
			for i := int64(0); i < job.Request.RenderSlices; i++ {
				sliceKey := getSliceKey(namespace, jobID, frame, i)
				data, err := d.redisClient.Get(ctx, sliceKey).Bytes()
				if err != nil {
					return nil, errors.Wrapf(err, "error getting slice %d (frame %d) for job %s from database", i, frame, jobID)
				}

				sliceJob := api.SliceJob{}
				if err := proto.Unmarshal(data, &sliceJob); err != nil {
					return nil, err
				}
				frameJob.SliceJobs = append(frameJob.SliceJobs, &sliceJob)
			}
		}
		job.FrameJobs = append(job.FrameJobs, &frameJob)
	}

	if err := resolveJob(ctx, &job); err != nil {
		return nil, err
	}

	// cache
	ttl := jobCacheTTL
	if job.Status == api.JobStatus_FINISHED {
		// cache finished jobs more aggressive
		ttl = time.Second * 300
	}
	cData, err := proto.Marshal(&job)
	if err != nil {
		return nil, err
	}

	if err := d.SetCacheObject(ctx, cacheKey, cData, ttl); err != nil {
		logrus.WithError(err).Warnf("error caching object for job %s", job.ID)
	}

	return &job, nil
}

func (d *Datastore) GetJobLog(ctx context.Context, jobID string) (*api.JobLog, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetJobLog")
	defer span.End()

	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	switch job.Status {
	case api.JobStatus_QUEUED, api.JobStatus_RENDERING:
		return nil, nil
	}

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	span.SetAttributes(attribute.String("namespace", namespace))

	logPath := getJobLogPath(namespace, jobID)
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

// ResolveJob resolves a job from all frame and slice jobs and stores the result in the database
func (d *Datastore) ResolveJob(ctx context.Context, jobID string) error {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "ResolveJob")
	defer span.End()

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	span.SetAttributes(attribute.String("namespace", namespace))

	jobKey := getJobKey(namespace, jobID)
	data, err := d.redisClient.Get(ctx, jobKey).Bytes()
	if err != nil {
		return errors.Wrapf(err, "error getting job %s from database to resolve (%s)", jobID, jobKey)
	}

	job := api.Job{}
	if err := proto.Unmarshal(data, &job); err != nil {
		return err
	}

	keys, err := d.redisClient.Keys(ctx, getJobKey(namespace, "*")).Result()
	if err != nil {
		return err
	}

	// TODO: ??

	jobs := []*api.Job{}
	for _, k := range keys {
		parts := strings.Split(k, "/")
		if len(parts) != 5 {
			logrus.Errorf("invalid key format: %s", k)
			continue
		}
		jobID := parts[3]
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
			frameKey := getFrameKey(namespace, jobID, f.RenderFrame)
			data, err := d.redisClient.Get(ctx, frameKey).Bytes()
			if err != nil {
				return errors.Wrapf(err, "error getting frame %d for job %s from database", f.RenderFrame, jobID)
			}

			frameJob := api.FrameJob{}
			if err := proto.Unmarshal(data, &frameJob); err != nil {
				return err
			}

			frameJobs = append(frameJobs, &frameJob)
			continue
		}
		// load slices
		sliceJobs := []*api.SliceJob{}
		for _, slice := range f.SliceJobs {
			sliceKey := getSliceKey(namespace, jobID, slice.RenderFrame, slice.RenderSliceIndex)
			data, err := d.redisClient.Get(ctx, sliceKey).Bytes()
			if err != nil {
				return errors.Wrapf(err, "error getting slice %d (frame %d) for job %s from database", slice.RenderSliceIndex, f.RenderFrame, jobID)
			}

			sliceJob := api.SliceJob{}
			if err := proto.Unmarshal(data, &sliceJob); err != nil {
				return err
			}

			sliceJobs = append(sliceJobs, &sliceJob)
		}
		f.SliceJobs = sliceJobs
		frameJobs = append(frameJobs, f)
	}

	job.FrameJobs = frameJobs

	if err := resolveJob(ctx, &job); err != nil {
		return err
	}

	if err := d.UpdateJob(ctx, jobID, &job); err != nil {
		return err
	}

	return nil
}

// UpdateJob updates the status of a job
func (d *Datastore) UpdateJob(ctx context.Context, jobID string, o interface{}) error {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "UpdateJob")
	defer span.End()

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("jobID", jobID))

	dbKey := ""
	var data []byte

	switch v := o.(type) {
	case *api.Job:
		dbKey = getJobKey(namespace, jobID)
		d, err := proto.Marshal(v)
		if err != nil {
			return err
		}
		data = d
	case *api.FrameJob:
		dbKey = getFrameKey(namespace, jobID, v.RenderFrame)
		d, err := proto.Marshal(v)
		if err != nil {
			return err
		}
		data = d
	case *api.SliceJob:
		dbKey = getSliceKey(namespace, jobID, v.RenderFrame, v.RenderSliceIndex)
		d, err := proto.Marshal(v)
		if err != nil {
			return err
		}
		data = d
	default:
		return fmt.Errorf("job type %+T is unknown", v)
	}

	if err := d.redisClient.Set(ctx, dbKey, data, 0).Err(); err != nil {
		return errors.Wrapf(err, "error saving job %s in database", jobID)
	}

	return nil
}

// DeleteJob deletes a job and all related files from the datastore
func (d *Datastore) DeleteJob(ctx context.Context, jobID string) error {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "DeleteJob")
	defer span.End()

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)

	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("jobID", jobID))

	// delete from db
	keys := []string{
		getJobKey(namespace, jobID),
	}

	frameKeys, err := d.redisClient.Keys(ctx, getFrameKey(namespace, jobID, "*")).Result()
	if err != nil {
		return err
	}

	keys = append(keys, frameKeys...)

	sliceKeys, err := d.redisClient.Keys(ctx, getSliceKey(namespace, jobID, "*", "*")).Result()
	if err != nil {
		return err
	}
	keys = append(keys, sliceKeys...)

	if err := d.redisClient.Del(ctx, keys...).Err(); err != nil {
		return err
	}

	// delete project
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(fynca.S3ProjectPath, jobID),
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
		Prefix:    path.Join(fynca.S3RenderPath, jobID),
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

func getStoragePath(namespace, jobID string) string {
	return path.Join(namespace, fynca.S3ProjectPath, jobID)
}

func getJobPath(namespace, jobID string) string {
	jobPath := path.Join(getStoragePath(namespace, jobID), fynca.S3JobPath)
	return jobPath
}

func getJobLogPath(namespace, jobID string) string {
	return path.Join(getStoragePath(namespace, jobID), fynca.S3JobLogPath)
}

func getFrameJobPath(namespace, jobID string, frame int64) string {
	return path.Join(getStoragePath(namespace, jobID), fmt.Sprintf("frame_%04d_%s", frame, fynca.S3JobPath))
}

func getSliceJobPath(namespace, jobID string, frame int64, renderSliceIndex int64) string {
	return fmt.Sprintf("%s.%d", getFrameJobPath(namespace, jobID, frame), renderSliceIndex)
}

func getJobKey(namespace, jobID string) string {
	return path.Join(dbPrefix, namespace, "jobs", jobID, "job")
}

func getFrameKey(namespace, jobID string, frame interface{}) string {
	return path.Join(dbPrefix, namespace, "frames", jobID, fmt.Sprintf("%v", frame))
}

func getSliceKey(namespace, jobID string, frame interface{}, sliceIndex interface{}) string {
	return path.Join(dbPrefix, namespace, "slices", jobID, fmt.Sprintf("%v", frame), fmt.Sprintf("%v", sliceIndex))
}

// WARNING: dragons ahead....
//
// resolveJob "merges" and "resolves" all frame and slice jobs
// in the passed job and will set the status at the least progressed and set the
// render time at the longest time of the individual frames and slices
func resolveJob(ctx context.Context, job *api.Job) error {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "resolveJob")
	defer span.End()

	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("jobID", job.ID))

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
		span.RecordError(err)
		return err
	}
	job.Duration = renderTime

	return nil
}

func getJobCacheKey(jobID string) string {
	return fmt.Sprintf("%s", jobID)
}
