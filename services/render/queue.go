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

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	nomadapi "github.com/hashicorp/nomad/api"
	minio "github.com/minio/minio-go/v7"
	cs "github.com/mitchellh/copystructure"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	storageProjectBucketName = "projects"
	s3RenderDirectory        = "render"
	renderSliceTemplate      = `
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

	// save to minio
	jobFileName, err := getStorageJobPath(jobReq)
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

	// TODO: queue to nomad

	logrus.Debug("closing stream")
	if err := stream.SendAndClose(&api.QueueJobResponse{
		UUID: jobReq.UUID,
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	logrus.Debugf("received uploaded job %s of size %d", jobName, jobSize)
	if err := s.queueNomadJob(jobReq); err != nil {
		return status.Errorf(codes.Internal, "error queueing compute job: %s", err)
	}
	logrus.Infof("queued job %s (%s)", jobName, jobReq.UUID)
	return nil
}

// this takes care of creating and submitting jobs for rendering to nomad
// the details for each task group and task are heavily coupled to the worker
// and compositor as the worker containers need configuration for rendering
// if anything is changed here check the /worker/render.sh and /cmd/finca-compositor
// subdirectories for corresponding updates
func (s *service) queueNomadJob(req *api.JobRequest) error {
	jobID := s.getJobID(req)
	jobType := "batch"
	frameCount := int64(1)
	if req.GetRenderEndFrame() > 0 {
		frameCount = (req.GetRenderEndFrame() - req.GetRenderStartFrame()) + int64(1)
	}

	jobPriority := req.RenderPriority
	if jobPriority == 0 {
		jobPriority = int64(s.config.JobPriority)
	}
	job := nomadapi.NewBatchJob("", jobID, s.config.NomadRegion, int(jobPriority))
	job.Region = &s.config.NomadRegion
	job.Namespace = &s.config.NomadNamespace
	job.ID = &jobID
	job.Type = &jobType
	job.Datacenters = s.config.NomadDatacenters

	runtimeConfig := map[string]interface{}{
		"image": s.config.JobImage,
	}

	taskEnv := map[string]string{
		"WORK_DIR":             "/local",
		"PROJECT_NAME":         req.GetName(),
		"PROJECT_ID":           jobID,
		"S3_ACCESS_KEY_ID":     s.config.S3AccessID,
		"S3_ACCESS_KEY_SECRET": s.config.S3AccessKey,
		"S3_ENDPOINT":          s.config.S3Endpoint,
		"S3_OUTPUT_DIR":        s.getJobOutputDir(),
		"OPTIX_CACHE_PATH":     "/data/finca_optix_cache",
		"RENDER_START_FRAME":   fmt.Sprintf("%d", req.GetRenderStartFrame()),
		"RENDER_END_FRAME":     fmt.Sprintf("%d", req.GetRenderEndFrame()),
		"RENDER_SAMPLES":       fmt.Sprintf("%d", req.GetRenderSamples()),
		"RENDER_DEVICE":        "CPU",
	}

	affinities := []*nomadapi.Affinity{}
	// run jobs on unique hosts for best performance
	constraints := []*nomadapi.Constraint{
		{
			Operand: "distinct_hosts",
			RTarget: "true",
		},
	}
	if req.GetRenderUseGPU() {
		constraints = append(constraints, nomadapi.NewConstraint("${node.class}", "=", "gpu"))
		runtimeConfig["gpus"] = []int{0}
		runtimeConfig["mounts"] = map[string]interface{}{
			"type":    "bind",
			"source":  "/tmp",
			"target":  "/data",
			"options": []string{"rbind", "rw"},
		}
		taskEnv["RENDER_DEVICE"] = "GPU"
	}

	taskName := "render"
	jobCount := int(frameCount)
	jobInterval := 20 * time.Second
	jobDelay := 5 * time.Second
	jobMigrate := true
	jobSticky := true
	jobArtifactScheme := "http"
	if s.config.S3UseSSL {
		jobArtifactScheme = "https"
	}
	jobFileName, err := getStorageJobPath(req)
	if err != nil {
		return err
	}
	jobArtifactSource := fmt.Sprintf("s3::%s://%s/%s", jobArtifactScheme, s.config.S3Endpoint, path.Join(s.config.S3Bucket, jobFileName))
	jobArtifactDestination := "local/project"
	jobArtifactMode := "any"
	jobCPU := int(req.CPU)
	jobMemory := int(req.Memory)
	if jobCPU == 0 {
		jobCPU = s.config.JobCPU
	}
	if jobMemory == 0 {
		jobMemory = s.config.JobMemory
	}

	// default tasks
	task := &nomadapi.Task{
		Name:   fmt.Sprintf("render-%s", req.GetName()),
		Driver: "containerd-driver",
		Config: runtimeConfig,
		Env:    taskEnv,
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
			CPU:      &jobCPU,
			MemoryMB: &jobMemory,
		},
	}
	taskGroups := []*nomadapi.TaskGroup{
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
			Tasks: []*nomadapi.Task{task},
		},
	}
	// if render slices are used create tasks for each slice
	if req.RenderSlices > 0 {
		logrus.Debugf("calculating %d slices", int(req.RenderSlices))
		slices, err := calculateRenderSlices(int(req.RenderSlices))
		if err != nil {
			return err
		}
		taskGroups = []*nomadapi.TaskGroup{}
		for i := 0; i < len(slices); i++ {
			slice := slices[i]
			// copy and override env to set slice region
			te, err := cs.Copy(taskEnv)
			if err != nil {
				return err
			}
			tEnv := te.(map[string]string)
			tEnv["RENDER_SLICE_MIN_X"] = fmt.Sprintf("%1f", slice.MinX)
			tEnv["RENDER_SLICE_MAX_X"] = fmt.Sprintf("%1f", slice.MaxX)
			tEnv["RENDER_SLICE_MIN_Y"] = fmt.Sprintf("%1f", slice.MinY)
			tEnv["RENDER_SLICE_MAX_Y"] = fmt.Sprintf("%1f", slice.MaxY)
			tName := fmt.Sprintf("%s-slice-%d", taskName, i)
			st, err := cs.Copy(task)
			if err != nil {
				return err
			}
			// copy and override task to configure slice region
			sliceTask := st.(*nomadapi.Task)
			// update env for slice env vars
			sliceTask.Env = tEnv
			taskGroups = append(taskGroups, []*nomadapi.TaskGroup{
				{
					Name:  &tName,
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
					Tasks: []*nomadapi.Task{sliceTask},
				},
			}...)
		}
		// composite task
		compositeTaskName := fmt.Sprintf("%s-composite", req.GetName())
		// copy and override task to configure compositing
		ct, err := cs.Copy(task)
		if err != nil {
			return err
		}
		compositeTask := ct.(*nomadapi.Task)
		ce, err := cs.Copy(taskEnv)
		if err != nil {
			return err
		}
		cEnv := ce.(map[string]string)
		cEnv["NOMAD_ADDR"] = s.config.NomadAddress
		cEnv["NOMAD_JOB_ID"] = jobID
		cEnv["NOMAD_NAMESPACE"] = s.config.NomadNamespace
		cEnv["RENDER_SLICES"] = fmt.Sprintf("%d", len(slices))
		cEnv["S3_BUCKET"] = s.config.S3Bucket
		cEnv["S3_RENDER_DIRECTORY"] = s3RenderDirectory
		cEnv["DEBUG"] = "true"
		compositeTask.Env = cEnv
		compositeTask.Lifecycle = &nomadapi.TaskLifecycle{
			Hook:    "poststop",
			Sidecar: false,
		}
		// override command for compositing
		compositeTask.Config["command"] = "/usr/local/bin/finca-compositor"
		taskGroups = append(taskGroups, []*nomadapi.TaskGroup{
			{
				Name:        &compositeTaskName,
				Constraints: []*nomadapi.Constraint{},
				Affinities:  []*nomadapi.Affinity{},
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
				Tasks: []*nomadapi.Task{compositeTask},
			},
		}...)
	}

	job.Affinities = affinities
	job.Constraints = constraints
	job.TaskGroups = taskGroups

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
	return path.Join(s.config.S3Bucket, s3RenderDirectory)
}

func getStorageJobPath(req *api.JobRequest) (string, error) {
	ext := ""
	switch strings.ToLower(req.ContentType) {
	case "application/zip":
		ext = "zip"
	case "application/octet-stream":
		// assume binary is blend; worker will fail if not
		ext = "blend"
	default:
		return "", fmt.Errorf("unknown content type: %s", req.ContentType)
	}
	objName := fmt.Sprintf("%s-%s.%s", req.UUID, req.GetName(), ext)
	return path.Join(storageProjectBucketName, objName), nil
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
