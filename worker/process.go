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
package worker

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/jobs/v1"
	"git.underland.io/ehazlett/fynca/pkg/tracing"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	// ErrRenderInProgress is returned when a render job is still running
	ErrRenderInProgress = errors.New("render still in progress")

	blenderConfigTemplate = `import bpy
import os

prop = bpy.context.preferences.addons['cycles'].preferences
prop.get_devices()

# set output path
filename = bpy.path.basename(bpy.data.filepath)
filename = os.path.splitext(filename)[0]

if filename:
    current_dir = os.getcwd()

{{ if .Request.RenderUseGPU }}
# TODO: detect OPTIX support and use
for compute_device_type in ('CUDA', 'OPENCL', 'NONE'):
    try:
        prop.compute_device_type = compute_device_type
        print("Enabled device " + compute_device_type)
        break
    except TypeError:
        pass
{{ else }}
prop.compute_device_type = 'NONE'
{{ end }}

for scene in bpy.data.scenes:
    scene.cycles.device = '{{ if .Request.RenderUseGPU }}GPU{{ else }}CPU{{ end }}'
    scene.cycles.samples = {{ .Request.RenderSamples }}
    # TODO: enable configurable render engines (i.e. BLENDER_EEVEE)
    scene.render.engine = '{{ .Request.RenderEngine }}'
    scene.render.resolution_x = {{ .Request.ResolutionX }}
    scene.render.resolution_y = {{ .Request.ResolutionY }}
    scene.render.resolution_percentage = {{ .Request.ResolutionScale }}
    scene.render.filepath = '{{ .OutputDir }}_'
    # TODO: make output format, color mode, etc. configurable
    scene.render.image_settings.file_format = 'PNG'
    scene.render.image_settings.color_mode ='RGBA'
    {{ if .SliceJob }}
    scene.render.border_min_x = {{ .SliceJob.RenderSliceMinX }}
    scene.render.border_max_x = {{ .SliceJob.RenderSliceMaxX }}
    scene.render.border_min_y = {{ .SliceJob.RenderSliceMinY }}
    scene.render.border_max_y = {{ .SliceJob.RenderSliceMaxY }}
    scene.render.use_border = True
    {{ end }}
`
)

type blenderConfig struct {
	Request   *api.JobRequest
	SliceJob  *api.SliceJob
	FrameJob  *api.FrameJob
	OutputDir string
}

func (w *Worker) processFrameJob(ctx context.Context, job *api.FrameJob) (*api.JobResult, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "ProcessFrameJob")
	defer span.End()

	span.SetAttributes(attribute.String("namespace", job.Request.Namespace))
	span.SetAttributes(attribute.String("name", job.Request.Name))
	span.SetAttributes(attribute.String("jobID", job.ID))
	span.SetAttributes(attribute.Int64("renderFrame", job.RenderFrame))

	logrus.Infof("processing frame job %s (%s)", job.ID, job.GetJobSource())
	w.jobLock.Lock()
	defer w.jobLock.Unlock()

	// check for gpu
	if job.GetRequest().RenderUseGPU && !w.gpuEnabled {
		logrus.Debug("skipping GPU job as worker does not have GPU")
		return nil, ErrGPUNotSupported
	}

	job.StartedAt = time.Now()
	job.Status = api.JobStatus_RENDERING
	wInfo, err := w.getWorkerInfo()
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	job.Worker = wInfo

	if err := w.ds.UpdateJob(ctx, job.GetID(), job); err != nil {
		span.RecordError(err)
		return nil, err
	}

	jobResult := &api.JobResult{
		Namespace: job.Request.Namespace,
		Result: &api.JobResult_FrameJob{
			FrameJob: job,
		},
		RenderFrame: job.RenderFrame,
	}

	// process
	var pErr error
	started := time.Now()
	if err := w.processJob(ctx, job, job.Request.Name); err != nil {
		pErr = err
		span.RecordError(err)
	}
	// update result status
	job.FinishedAt = time.Now()
	job.Status = api.JobStatus_FINISHED
	jobResult.Status = api.JobStatus_FINISHED
	jobResult.Duration = time.Now().Sub(started)
	if pErr != nil {
		jobResult.Error = pErr.Error()
		jobResult.Status = api.JobStatus_ERROR
		job.Status = api.JobStatus_ERROR
	}

	if err := w.ds.UpdateJob(ctx, job.GetID(), job); err != nil {
		span.RecordError(err)
		return nil, err
	}

	return jobResult, pErr
}

func (w *Worker) processSliceJob(ctx context.Context, job *api.SliceJob) (*api.JobResult, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "ProcessSliceJob")
	defer span.End()

	span.SetAttributes(attribute.String("namespace", job.Request.Namespace))
	span.SetAttributes(attribute.String("name", job.Request.Name))
	span.SetAttributes(attribute.String("jobID", job.ID))
	span.SetAttributes(attribute.Int64("renderFrame", job.RenderFrame))
	span.SetAttributes(attribute.Int64("renderSlice", job.RenderSliceIndex))

	logrus.Infof("processing slice job %s (%s)", job.ID, job.GetJobSource())
	w.jobLock.Lock()
	defer w.jobLock.Unlock()

	// check for gpu
	if job.GetRequest().RenderUseGPU && !w.gpuEnabled {
		logrus.Debug("skipping GPU job as worker does not have GPU")
		return nil, ErrGPUNotSupported
	}

	job.StartedAt = time.Now()
	job.Status = api.JobStatus_RENDERING
	wInfo, err := w.getWorkerInfo()
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	job.Worker = wInfo

	if err := w.ds.UpdateJob(ctx, job.ID, job); err != nil {
		span.RecordError(err)
		return nil, err
	}

	jobResult := &api.JobResult{
		Namespace: job.Request.Namespace,
		Result: &api.JobResult_SliceJob{
			SliceJob: job,
		},
		RenderFrame: job.RenderFrame,
	}
	// process
	var pErr error
	started := time.Now()
	if err := w.processJob(ctx, job, fmt.Sprintf("%s_slice_%d", job.Request.Name, job.RenderSliceIndex)); err != nil {
		pErr = err
		span.RecordError(err)
	}
	// update result status
	job.FinishedAt = time.Now()
	job.Status = api.JobStatus_FINISHED
	jobResult.Status = api.JobStatus_FINISHED
	jobResult.Duration = time.Now().Sub(started)
	if pErr != nil {
		jobResult.Error = pErr.Error()
		jobResult.Status = api.JobStatus_ERROR
		job.Status = api.JobStatus_ERROR
	}

	if err := w.ds.UpdateJob(ctx, job.GetID(), job); err != nil {
		span.RecordError(err)
		return nil, err
	}

	return jobResult, pErr
}

func (w *Worker) processJob(ctx context.Context, job api.RenderJob, outputPrefix string) error {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "ProcessJob")
	defer span.End()

	span.SetAttributes(attribute.Int64("renderSamples", job.GetRequest().RenderSamples))
	span.SetAttributes(attribute.Int64("renderResolutionX", job.GetRequest().ResolutionX))
	span.SetAttributes(attribute.Int64("renderResolutionY", job.GetRequest().ResolutionY))
	span.SetAttributes(attribute.Int64("renderResolutionScale", job.GetRequest().ResolutionScale))
	span.SetAttributes(attribute.Bool("renderUseGPU", job.GetRequest().RenderUseGPU))
	span.SetAttributes(attribute.String("renderEngine", job.GetRequest().RenderEngine.String()))

	// render with blender
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("fynca-%s", job.GetID()))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	logrus.Debugf("temp work dir: %s", tmpDir)

	// setup render dir
	outputDir := filepath.Join(tmpDir, "render")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	blenderOutputDir := getPythonOutputDir(filepath.Join(outputDir, outputPrefix))

	// fetch project source file
	mc, err := w.getMinioClient()
	if err != nil {
		return err
	}

	logrus.Debugf("getting job source %s", job.GetJobSource())
	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)
	span.SetAttributes(attribute.String("jobSource", job.GetJobSource()))

	_, gspan := tracing.StartSpan(ctx, "getJobSource")
	gspan.SetAttributes(attribute.String("bucket", w.config.S3Bucket))
	object, err := mc.GetObject(ctx, w.config.S3Bucket, path.Join(job.GetJobSource()), minio.GetObjectOptions{})
	if err != nil {
		return errors.Wrapf(err, "error getting job source %s", job.GetJobSource())
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, job.GetJobSource())), 0755); err != nil {
		return err
	}

	projectFile, err := os.Create(filepath.Join(tmpDir, job.GetJobSource()))
	if err != nil {
		return err
	}

	if _, err := io.Copy(projectFile, object); err != nil {
		return err
	}
	gspan.End()

	projectFilePath := projectFile.Name()

	logrus.Debugf("checking content type for %s", projectFile.Name())
	contentType, err := getContentType(projectFile)
	if err != nil {
		return errors.Wrapf(err, "error getting content type for %s", projectFile.Name())
	}

	switch contentType {
	case "application/zip":
		_, zspan := tracing.StartSpan(ctx, "contentTypeZip")
		zspan.SetAttributes(attribute.String("projectFilePath", projectFilePath))
		logrus.Debug("zip archive detected; extracting")
		a, err := zip.OpenReader(projectFilePath)
		if err != nil {
			return errors.Wrapf(err, "error reading zip archive %s", projectFilePath)
		}
		defer a.Close()

		for _, f := range a.File {
			fp := filepath.Join(tmpDir, f.Name)
			if !strings.HasPrefix(fp, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
				logrus.Warnf("invalid path for archive file %s", f.Name)
				continue
			}
			if f.FileInfo().IsDir() {
				if err := os.MkdirAll(fp, 0750); err != nil {
					return errors.Wrapf(err, "error creating dir from archive for %s", f.Name)
				}
				continue
			}

			if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
				return errors.Wrapf(err, "error creating parent dir from archive for %s", f.Name)
			}

			pf, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return errors.Wrapf(err, "error creating file from archive for %s", f.Name)
			}

			fa, err := f.Open()
			if err != nil {
				return errors.Wrapf(err, "error opening file in archive: %s", f.Name)
			}

			if _, err := io.Copy(pf, fa); err != nil {
				return errors.Wrapf(err, "error extracting file from archive: %s", f.Name)
			}
			pf.Close()
			fa.Close()
		}

		// find blender project
		projectFiles, err := filepath.Glob(filepath.Join(tmpDir, "*.blend"))
		if err != nil {
			return err
		}

		if len(projectFiles) == 0 {
			return errors.New("Blender project file (.blend) not found in zip archive")
		}

		projectFilePath = projectFiles[0]
		if len(projectFiles) > 1 {
			logrus.Warnf("multiple Blender (.blend) projects found; using first detected: %s", projectFilePath)
		}
		zspan.End()
	default:
	}

	bCfg := &blenderConfig{
		Request:   job.GetRequest(),
		OutputDir: blenderOutputDir,
	}
	if v, ok := job.(*api.SliceJob); ok {
		bCfg.SliceJob = v
	}
	blenderCfg, err := generateBlenderRenderConfig(bCfg)
	if err != nil {
		return err
	}

	logrus.Debugf("blender config: %+v", blenderCfg)

	blenderConfigPath := filepath.Join(tmpDir, "render.py")

	if err := ioutil.WriteFile(blenderConfigPath, []byte(blenderCfg), 0644); err != nil {
		return err
	}

	// lookup binary path
	// check config first and if not there attempt to resolve from PATH
	blenderBinaryPath := ""
	workerConfig := w.config.GetWorkerConfig(w.id)
	logrus.Debugf("worker config: %+v", workerConfig)
	for _, engine := range workerConfig.RenderEngines {
		logrus.Debugf("checking render engine config: %+v", engine)
		if strings.ToLower(engine.Name) == "blender" {
			logrus.Infof("using blender render path: %s", engine.Path)
			blenderBinaryPath = engine.Path
			break
		}
	}
	if blenderBinaryPath == "" {
		bp, err := exec.LookPath(blenderExecutableName)
		if err != nil {
			return err
		}
		blenderBinaryPath = bp
	}

	logrus.Debugf("using render engine %s", job.GetRequest().RenderEngine)

	args := []string{
		"-b",
		"--factory-startup",
		projectFilePath,
		"-P",
		blenderConfigPath,
		"--debug-python",
		"-f",
		fmt.Sprintf("%d", job.GetRenderFrame()),
	}
	args = append(args, blenderCommandPlatformArgs...)

	logrus.Debugf("blender args: %v", args)
	_, cspan := tracing.StartSpan(ctx, "workerJob.render")
	c := exec.CommandContext(ctx, blenderBinaryPath, args...)

	out, err := c.CombinedOutput()
	if err != nil {
		// copy log to tmp
		errLogName := fmt.Sprintf("%s-error.log", job.GetID())
		tmpLogPath := filepath.Join(os.TempDir(), errLogName)
		if err := os.WriteFile(tmpLogPath, out, 0644); err != nil {
			logrus.WithError(err).Errorf("unable to save error log file for job %s", job.GetID())
		}
		logrus.WithError(err).Errorf("error log available at %s", tmpLogPath)
		return errors.Wrap(err, string(out))
	}
	cspan.End()

	logrus.Debug(string(out))

	_, uspan := tracing.StartSpan(ctx, "upload")
	uspan.SetAttributes(attribute.String("outputDir", outputDir))
	logrus.Debugf("uploading output files from %s", outputDir)
	// upload results to minio
	files, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		logrus.Debugf("uploading output file %s", f)
		rf, err := os.Open(f)
		if err != nil {
			return err
		}
		defer rf.Close()

		fs, err := rf.Stat()
		if err != nil {
			return err
		}
		if fs.IsDir() {
			continue
		}

		if _, err := mc.PutObject(ctx, w.config.S3Bucket, path.Join(namespace, fynca.S3RenderPath, job.GetID(), filepath.Base(f)), rf, fs.Size(), minio.PutObjectOptions{ContentType: "application/octet-stream"}); err != nil {
			return err
		}
	}

	// upload blender log for render
	logFilename := fmt.Sprintf("%s_%04d.log", outputPrefix, job.GetRenderFrame())
	logFile, err := os.Create(filepath.Join(tmpDir, logFilename))
	if err != nil {
		return err
	}
	defer logFile.Close()

	if _, err := logFile.Write(out); err != nil {
		return err
	}

	if _, err := logFile.Seek(0, 0); err != nil {
		return err
	}

	fi, err := logFile.Stat()
	if err != nil {
		return err
	}

	if _, err := mc.PutObject(ctx, w.config.S3Bucket, path.Join(namespace, fynca.S3RenderPath, job.GetID(), logFilename), logFile, fi.Size(), minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		return err
	}
	uspan.End()

	// check for render slice job; if so, check s3 for number of renders. if matches render slices
	// assume project is complete and composite images into single result
	if job.GetRequest().RenderSlices > 0 {
		if err := w.compositeRender(ctx, namespace, job, job.GetRenderFrame(), outputDir); err != nil {
			if err != ErrRenderInProgress {
				return err
			}
		}
	}

	return nil
}

func (w *Worker) compositeRender(ctx context.Context, namespace string, job api.RenderJob, frame int64, outputDir string) error {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "compositeRender")
	defer span.End()

	mc, err := w.getMinioClient()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("fynca-composite-%s", job.GetID()))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	logrus.Debugf("temp composite dir: %s", tmpDir)
	objCh := mc.ListObjects(ctx, w.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    fmt.Sprintf("%s/%s/%s", namespace, fynca.S3RenderPath, job.GetID()),
		Recursive: true,
	})
	slices := []string{}
	for o := range objCh {
		if o.Err != nil {
			return err
		}

		// filter logs
		if filepath.Ext(o.Key) == ".log" {
			continue
		}
		slices = append(slices, o.Key)
	}

	if int64(len(slices)) < job.GetRequest().RenderSlices {
		logrus.Infof("render for project %s is still in progress; skipping compositing", job.GetID())
		return ErrRenderInProgress
	}

	logrus.Infof("detected complete render; compositing frame %d for job %s", frame, job.GetID())

	renderStartFrame := job.GetRequest().RenderStartFrame
	renderEndFrame := job.GetRequest().RenderEndFrame
	span.SetAttributes(attribute.Int64("renderStartFrame", renderStartFrame))
	span.SetAttributes(attribute.Int64("renderEndFrame", renderEndFrame))
	if renderEndFrame == 0 {
		renderEndFrame = renderStartFrame
	}
	data, err := w.ds.CompositeFrameRender(ctx, job.GetID(), frame)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(data)

	// sync final composite back to s3
	objectName := path.Join(namespace, fynca.S3RenderPath, job.GetID(), fmt.Sprintf("%s_%04d.png", job.GetRequest().Name, frame))
	logrus.Infof("uploading composited frame %d (%s) to bucket %s (%d bytes)", frame, objectName, w.config.S3Bucket, buf.Len())
	// TODO: support other image types
	if _, err := mc.PutObject(ctx, w.config.S3Bucket, objectName, buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: "image/png"}); err != nil {
		return errors.Wrapf(err, "error uploading final composite %s", objectName)
	}

	return nil
}

func generateBlenderRenderConfig(cfg *blenderConfig) (string, error) {
	t, err := template.New("blender").Parse(blenderConfigTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}

func getContentType(f *os.File) (string, error) {
	buffer := make([]byte, 512)
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	_, err := f.Read(buffer)
	if err != nil {
		return "", err
	}
	contentType := http.DetectContentType(buffer)
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	return contentType, nil
}
