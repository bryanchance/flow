package render

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	nomadapi "github.com/hashicorp/nomad/api"
	minio "github.com/minio/minio-go/v7"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	storageProjectBucketName = "projects"
)

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
			logrus.Debug("EOF stopping")
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

	// generate id
	id := uuid.NewV4()

	// TODO: save to minio
	jobFileName := getStorageJobPath(jobReq)
	jobContentType := "application/zip"

	logrus.Debugf("saving %s to storage", jobFileName)
	ctx := context.Background()
	jobStorageInfo, err := s.storageClient.FPutObject(ctx, s.config.S3Bucket, jobFileName, tmpJobFile.Name(), minio.PutObjectOptions{ContentType: jobContentType})
	if err != nil {
		return status.Errorf(codes.Internal, "error saving job to storage: %s", err)
	}

	logrus.Debugf("saved job %s to storage service (%d bytes)", jobName, jobStorageInfo.Size)

	// TODO: queue to nomad

	logrus.Debug("closing stream")
	if err := stream.SendAndClose(&api.QueueJobResponse{
		UUID: id.String(),
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	logrus.Debugf("received uploaded job %s of size %d", jobName, jobSize)
	if err := s.queueNomadJob(jobReq); err != nil {
		return status.Errorf(codes.Internal, "error queueing compute job: %s", err)
	}
	logrus.Infof("queued job %s (%s)", jobName, id)
	return nil
}

func (s *service) queueNomadJob(req *api.JobRequest) error {
	jobID := s.getJobID(req)
	jobType := "batch"
	frameCount := int64(1)
	if req.GetRenderEndFrame() > 0 {
		frameCount = (req.GetRenderEndFrame() - req.GetRenderStartFrame()) + int64(1)
	}
	job := nomadapi.NewBatchJob("", jobID, s.config.NomadRegion, s.config.JobPriority)
	job.ID = &jobID
	job.Type = &jobType
	job.Datacenters = s.config.NomadDatacenters

	runtimeConfig := map[string]interface{}{
		"image": s.config.JobImage,
	}

	if req.GetRenderUseGPU() {
		constraint := nomadapi.NewConstraint("${node.class}", "=", "gpu")
		job.Constraints = []*nomadapi.Constraint{constraint}

		runtimeConfig["gpus"] = []int{0}
		runtimeConfig["mounts"] = map[string]interface{}{
			"type":    "bind",
			"source":  "/tmp",
			"target":  "/data",
			"options": []string{"rbind", "rw"},
		}
	}

	taskName := "render"
	jobCount := int(frameCount)
	jobInterval := 5 * time.Second
	jobDelay := 10 * time.Second
	jobMigrate := true
	jobSticky := true
	jobArtifactScheme := "http"
	if s.config.S3UseSSL {
		jobArtifactScheme = "https"
	}
	jobArtifactSource := fmt.Sprintf("s3::%s://%s/%s", jobArtifactScheme, s.config.S3Endpoint, path.Join(s.config.S3Bucket, getStorageJobPath(req)))
	jobArtifactDestination := "local/project"
	jobArtifactMode := "any"

	job.TaskGroups = []*nomadapi.TaskGroup{
		{
			Name:  &taskName,
			Count: &jobCount,
			ReschedulePolicy: &nomadapi.ReschedulePolicy{
				Attempts: &s.config.JobMaxAttempts,
				Interval: &jobInterval,
				Delay:    &jobDelay,
			},
			EphemeralDisk: &nomadapi.EphemeralDisk{
				Migrate: &jobMigrate,
				Sticky:  &jobSticky,
			},
			Networks: []*nomadapi.NetworkResource{
				{
					Mode: "cni/default",
				},
			},
			Services: []*nomadapi.Service{
				{
					Name:        "render",
					AddressMode: "alloc",
				},
			},
			Tasks: []*nomadapi.Task{
				{
					Name:   fmt.Sprintf("render-%s", req.GetName()),
					Driver: "containerd-driver",
					Config: runtimeConfig,
					Env: map[string]string{
						"WORK_DIR":             "/local",
						"PROJECT_NAME":         jobID,
						"S3_ACCESS_KEY_ID":     s.config.S3AccessID,
						"S3_ACCESS_KEY_SECRET": s.config.S3AccessKey,
						"S3_ENDPOINT":          s.config.S3Endpoint,
						"S3_BUCKET":            s.getJobOutputDir(),
						"OPTIX_CACHE_PATH":     "/data/finca_optix_cache",
						"RENDER_START_FRAME":   fmt.Sprintf("%d", req.GetRenderStartFrame()),
					},
					Artifacts: []*nomadapi.TaskArtifact{
						{
							GetterSource: &jobArtifactSource,
							RelativeDest: &jobArtifactDestination,
							GetterMode:   &jobArtifactMode,
							GetterOptions: map[string]string{
								"aws_access_key_id":     s.config.S3AccessID,
								"aws_access_key_secret": s.config.S3AccessKey,
							},
						},
					},
					RestartPolicy: &nomadapi.RestartPolicy{
						Attempts: &s.config.JobMaxAttempts,
						Interval: &jobInterval,
						Delay:    &jobDelay,
					},
					Resources: &nomadapi.Resources{
						CPU:      &s.config.JobCPU,
						MemoryMB: &s.config.JobMemory,
					},
				},
			},
		},
	}

	// TODO register job
	resp, _, err := s.computeClient.Jobs().Register(job, nil)
	if err != nil {
		return err
	}

	logrus.Infof("registered job %s (%s)", req.GetName(), resp.EvalID)

	return nil
}

func (s *service) getJobID(req *api.JobRequest) string {
	return fmt.Sprintf("%s%s-%d", s.config.JobPrefix, req.GetName(), time.Now().Unix())
}

func (s *service) getJobOutputDir() string {
	return path.Join(s.config.S3Bucket, "render")
}

func getStorageJobPath(req *api.JobRequest) string {
	return path.Join(storageProjectBucketName, req.GetName()+".zip")
}
