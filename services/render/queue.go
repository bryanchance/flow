package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	minio "github.com/minio/minio-go/v7"
	cs "github.com/mitchellh/copystructure"
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

func (s *service) QueueJob(stream api.Render_QueueJobServer) error {
	logrus.Debug("processing queue request")
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Unknown, "error receiving job: %s", err)
	}

	var (
		jobReq  = req.GetRequest()
		buf     = bytes.Buffer{}
		jobName = jobReq.GetName()
		jobSize = 0
		jobID   = uuid.NewV4().String()
	)

	logrus.Debugf("queueing job %+v", jobReq)
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return status.Errorf(codes.Unknown, "error receiving job content: %s", err)
		}
		c := req.GetChunkData()
		if err == io.EOF {
			break
		}
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
	tmpJobFile, err := os.CreateTemp("", "finca-job-")
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
	ctx := context.Background()
	jobStorageInfo, err := s.storageClient.FPutObject(ctx, s.config.S3Bucket, jobFileName, tmpJobFile.Name(), minio.PutObjectOptions{ContentType: jobReq.ContentType})
	if err != nil {
		return status.Errorf(codes.Internal, "error saving job to storage: %s", err)
	}

	logrus.Debugf("saved job %s to storage service (%d bytes)", jobName, jobStorageInfo.Size)

	logrus.Debug("closing stream")
	if err := stream.SendAndClose(&api.QueueJobResponse{
		UUID: jobID,
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	logrus.Debugf("received uploaded job %s of size %d", jobName, jobSize)
	if err := s.queueJob(jobID, jobReq); err != nil {
		return status.Errorf(codes.Internal, "error queueing compute job: %s", err)
	}
	logrus.Infof("queued job %s (%s)", jobName, jobID)
	return nil
}

func (s *service) queueJob(jobID string, req *api.JobRequest) error {
	logrus.Infof("queueing job %s", jobID)

	js, err := s.natsClient.JetStream()
	if err != nil {
		return err
	}

	subjectName := s.config.NATSJobSubject
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

	for i := req.RenderStartFrame; i <= req.RenderEndFrame; i++ {
		job := &api.Job{
			ID:               jobID,
			Request:          req,
			JobSource:        jobSourceFileName,
			RenderFrame:      i,
			RenderSliceIndex: -1,
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
				te, err := cs.Copy(job)
				if err != nil {
					return err
				}
				sliceJob := te.(*api.Job)
				sliceJob.RenderSliceIndex = int64(i)
				sliceJob.RenderSliceMinX = float32(slice.MinX)
				sliceJob.RenderSliceMaxX = float32(slice.MaxX)
				sliceJob.RenderSliceMinY = float32(slice.MinY)
				sliceJob.RenderSliceMaxY = float32(slice.MaxY)
				// queue slice job
				sliceData, err := json.Marshal(sliceJob)
				if err != nil {
					return err
				}
				logrus.Debugf("publishing job slice %s (%d)", job.ID, i)
				js.Publish(subjectName, sliceData)
			}

			// slices queued; continue to next frame
			continue
		}

		// not using slices; publish job for full frame render
		logrus.Debugf("publishing job to %s", subjectName)

		jobData, err := json.Marshal(job)
		if err != nil {
			return err
		}
		js.Publish(subjectName, jobData)
	}

	return nil
}

func (s *service) getJobID(req *api.JobRequest) string {
	return fmt.Sprintf("%s%s-%d", s.config.JobPrefix, req.GetName(), time.Now().Unix())
}

func (s *service) getSubjectName(req *api.JobRequest) string {
	return fmt.Sprintf("%s.%s", s.config.NATSJobSubject, s.getJobID(req))
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
	objName := fmt.Sprintf("%s-%s.%s", jobID, req.GetName(), ext)
	return path.Join(finca.S3ProjectPath, objName), nil
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
