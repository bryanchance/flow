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
package render

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/jobs/v1"
	"github.com/gogo/protobuf/proto"
	minio "github.com/minio/minio-go/v7"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	renderSliceTemplate = `
export RENDER_SLICE_MIN_X=%1f
export RENDER_SLICE_MAX_X=%1f
export RENDER_SLICE_MIN_Y=%1f
export RENDER_SLICE_MAX_Y=%1f
`
)

type renderSlice struct {
	MinX float64
	MaxX float64
	MinY float64
	MaxY float64
}

func (s *service) QueueJob(stream api.Jobs_QueueJobServer) error {
	logrus.Debug("processing queue request")
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Unknown, "error receiving job: %s", err)
	}

	ctx := stream.Context()

	var (
		jobReq    = req.GetRequest()
		buf       = bytes.Buffer{}
		jobName   = jobReq.GetName()
		jobSize   = 0
		jobID     = uuid.NewV4().String()
		namespace = ctx.Value(fynca.CtxNamespaceKey).(string)
	)

	// override job request namespace with context passed
	jobReq.Namespace = namespace

	logrus.Debugf("queueing job %+v", jobReq)
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return status.Errorf(codes.Unknown, "error receiving job content: %s", err)
		}
		if err == io.EOF {
			break
		}
		c := req.GetChunkData()
		if err != nil {
			logrus.Error(err)
			return status.Errorf(codes.Unknown, "error receiving job: %s", err)
		}

		jobSize += len(c)

		if _, err := buf.Write(c); err != nil {
			logrus.Error(err)
			return status.Errorf(codes.Internal, "error saving job data: %s", err)
		}
	}

	// save to tmpfile to upload to s3
	tmpJobFile, err := os.CreateTemp("", "fynca-job-")
	if err != nil {
		return err
	}
	if _, err := buf.WriteTo(tmpJobFile); err != nil {
		return err
	}
	tmpJobFile.Close()

	defer os.Remove(tmpJobFile.Name())

	// save to minio
	jobFileName, err := getStorageJobPath(jobID, jobReq)
	if err != nil {
		return err
	}

	logrus.Debugf("saving %s to storage", jobFileName)
	jobStorageInfo, err := s.storageClient.FPutObject(ctx, s.config.S3Bucket, jobFileName, tmpJobFile.Name(), minio.PutObjectOptions{ContentType: jobReq.ContentType})
	if err != nil {
		return status.Errorf(codes.Internal, "error saving job to storage: %s", err)
	}

	logrus.Debugf("saved job %s to storage service (%d bytes)", jobName, jobStorageInfo.Size)

	if err := stream.SendAndClose(&api.QueueJobResponse{
		UUID: jobID,
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	logrus.Debugf("received uploaded job %s of size %d", jobName, jobSize)
	if err := s.queueJob(ctx, jobID, jobReq); err != nil {
		return status.Errorf(codes.Internal, "error queueing compute job: %s", err)
	}

	logrus.Infof("queued job %s (%s)", jobName, jobID)
	return nil
}

func (s *service) queueJob(ctx context.Context, jobID string, req *api.JobRequest) error {
	logrus.Infof("queueing job %s", jobID)

	js, err := s.natsClient.JetStream()
	if err != nil {
		return err
	}

	subjectName := s.config.NATSJobStreamName
	jobSourceFileName, err := getStorageJobPath(jobID, req)
	if err != nil {
		return err
	}

	if req.RenderStartFrame == 0 {
		req.RenderStartFrame = 1
	}
	if req.RenderEndFrame == 0 {
		req.RenderEndFrame = req.RenderStartFrame
	}

	jobQueueSubject := getJobQueueSubject(req)

	job := &api.Job{
		CreatedAt: time.Now(),
		ID:        jobID,
		Request:   req,
		JobSource: jobSourceFileName,
	}
	for frame := req.RenderStartFrame; frame <= req.RenderEndFrame; frame++ {
		frameJob := &api.FrameJob{
			ID:          jobID,
			Request:     req,
			JobSource:   jobSourceFileName,
			RenderFrame: int64(frame),
		}

		// queue slices
		// if render slices are used create tasks for each slice
		if req.RenderSlices > 0 {
			logrus.Debugf("calculating %d slices", int(req.RenderSlices))
			slices, err := calculateRenderSlices(int(req.RenderSlices))
			if err != nil {
				return err
			}
			for i := 0; i < len(slices); i++ {
				slice := slices[i]
				// copy and override env to set slice region
				sliceJob := &api.SliceJob{
					ID:               jobID,
					Request:          req,
					JobSource:        jobSourceFileName,
					RenderSliceIndex: int64(i),
					RenderSliceMinX:  float32(slice.MinX),
					RenderSliceMaxX:  float32(slice.MaxX),
					RenderSliceMinY:  float32(slice.MinY),
					RenderSliceMaxY:  float32(slice.MaxY),
					RenderFrame:      int64(frame),
				}
				// queue slice job
				data, err := proto.Marshal(&api.WorkerJob{
					ID: jobID,
					Job: &api.WorkerJob_SliceJob{
						SliceJob: sliceJob,
					},
				})
				if err != nil {
					return err
				}
				logrus.Debugf("publishing slice job %s (frame:%d slice:%d)", jobID, frame, i)
				ack, err := js.Publish(jobQueueSubject, data)
				if err != nil {
					return err
				}
				sliceJob.SequenceID = ack.Sequence
				sliceJob.Status = api.JobStatus_QUEUED
				sliceJob.QueuedAt = time.Now()

				if err := s.ds.UpdateJob(ctx, jobID, sliceJob); err != nil {
					return err
				}
				frameJob.SliceJobs = append(frameJob.SliceJobs, sliceJob)
			}

			job.FrameJobs = append(job.FrameJobs, frameJob)
			if err := s.ds.UpdateJob(ctx, jobID, frameJob); err != nil {
				return err
			}

			// slices queued; continue to next frame
			continue
		}

		// not using slices; publish job for full frame render
		logrus.Debugf("publishing frame job %s (%d) to %s", jobID, frame, subjectName)

		data, err := proto.Marshal(&api.WorkerJob{
			ID: jobID,
			Job: &api.WorkerJob_FrameJob{
				FrameJob: frameJob,
			},
		})
		if err != nil {
			return err
		}

		ack, err := js.Publish(jobQueueSubject, data)
		if err != nil {
			return err
		}

		frameJob.SequenceID = ack.Sequence
		frameJob.Status = api.JobStatus_QUEUED
		frameJob.QueuedAt = time.Now()
		if err := s.ds.UpdateJob(ctx, jobID, frameJob); err != nil {
			return err
		}
		job.FrameJobs = append(job.FrameJobs, frameJob)
	}
	// update parent job
	job.Status = api.JobStatus_QUEUED
	job.QueuedAt = time.Now()
	if err := s.ds.UpdateJob(ctx, jobID, job); err != nil {
		return err
	}

	return nil
}

func (s *service) getJobID(req *api.JobRequest) string {
	return fmt.Sprintf("%s%s-%d", s.config.JobPrefix, req.GetName(), time.Now().Unix())
}

func (s *service) getSubjectName(req *api.JobRequest) string {
	return fmt.Sprintf("%s.%s", s.config.NATSJobStreamName, s.getJobID(req))
}

func getStorageJobPath(jobID string, req *api.JobRequest) (string, error) {
	ext := ""
	switch strings.ToLower(req.ContentType) {
	case "application/zip":
		ext = "zip"
	case "application/x-gzip":
		ext = "gz"
	case "application/octet-stream":
		// assume binary is blend; worker will fail if not
		ext = "blend"
	default:
		return "", fmt.Errorf("unknown content type: %s", req.ContentType)
	}
	objName := fmt.Sprintf("%s-%s.%s", jobID, req.Name, ext)
	return path.Join(getStoragePath(req.Namespace, jobID), objName), nil
}

func getJobQueueSubject(req *api.JobRequest) string {
	if req.Priority == api.JobPriority_URGENT {
		return fynca.QueueSubjectJobPriorityUrgent
	}

	// set to animation if more than 1 frame requested
	if (req.RenderEndFrame - req.RenderStartFrame) > 0 {
		return fynca.QueueSubjectJobPriorityAnimation
	}

	if req.Priority == api.JobPriority_LOW {
		return fynca.QueueSubjectJobPriorityLow
	}

	return fynca.QueueSubjectJobPriorityNormal
}

func getStoragePath(namespace, jobID string) string {
	return path.Join(namespace, fynca.S3ProjectPath, jobID)
}

func calculateRenderSlices(workers int) ([]renderSlice, error) {
	// ensure even slices
	if (workers % 2) != 0 {
		workers++
	}
	offset := float64(1.0) / float64(workers)
	minX := 0.0
	maxX := offset
	minY := 0.0
	maxY := 1.0
	results := []renderSlice{
		{
			MinX: minX,
			MaxX: maxX,
			MinY: minY,
			MaxY: maxY,
		},
	}
	for i := 1; i < workers; i++ {
		minX += offset
		maxX += offset
		results = append(results, renderSlice{
			MinX: minX,
			MaxX: maxX,
			MinY: minY,
			MaxY: maxY,
		})
	}

	// adjust last tile
	results[len(results)-1].MaxX = 1.0
	return results, nil
}
