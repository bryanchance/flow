package worker

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
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

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
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
    # TODO: enable configurable render engines (i.e. EEVEE)
    scene.render.engine = 'CYCLES'
    scene.render.resolution_x = {{ .Request.ResolutionX }}
    scene.render.resolution_y = {{ .Request.ResolutionY }}
    scene.render.resolution_percentage = {{ .Request.ResolutionScale }}
    scene.render.filepath = '{{ .OutputDir }}_'
    # TODO: make output format, color mode, etc. configurable
    scene.render.image_settings.file_format = 'PNG'
    scene.render.image_settings.color_mode ='RGBA'
    {{ if gt .Request.RenderSlices 0 }}
    scene.render.border_min_x = {{ .RenderSliceMinX }}
    scene.render.border_max_x = {{ .RenderSliceMaxX }}
    scene.render.border_min_y = {{ .RenderSliceMinY }}
    scene.render.border_max_y = {{ .RenderSliceMaxY }}
    scene.render.use_border = True
    {{ end }}
`
)

func (w *Worker) processJob(ctx context.Context, job *api.Job) (*api.JobStatus, error) {
	logrus.Infof("processing job %s (%s)", job.ID, job.JobSource)

	start := time.Now()

	// render with blender
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("finca-%s", job.ID))
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	logrus.Debugf("temp work dir: %s", tmpDir)

	// TODO: defer job status

	// setup render dir
	outputDir := filepath.Join(tmpDir, "render")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}
	outputPrefix := fmt.Sprintf("%s", job.Request.Name)
	if job.Request.RenderSlices > 0 {
		outputPrefix += fmt.Sprintf("_slice-%d", job.RenderSliceIndex)
	}
	job.OutputDir = getPythonOutputDir(filepath.Join(outputDir, outputPrefix))

	// fetch project source file
	mc, err := w.getMinioClient()
	if err != nil {
		return nil, err
	}

	object, err := mc.GetObject(ctx, w.config.S3Bucket, job.JobSource, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, job.JobSource)), 0755); err != nil {
		return nil, err
	}

	projectFile, err := os.Create(filepath.Join(tmpDir, job.JobSource))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(projectFile, object); err != nil {
		return nil, err
	}

	projectFilePath := projectFile.Name()

	logrus.Debugf("checking content type for %s", projectFile.Name())
	contentType, err := getContentType(projectFile)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting content type for %s", projectFile.Name())
	}

	switch contentType {
	case "application/zip":
		logrus.Debug("zip archive detected; extracting")
		a, err := zip.OpenReader(projectFilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading zip archive %s", projectFilePath)
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
					return nil, errors.Wrapf(err, "error creating dir from archive for %s", f.Name)
				}
				continue
			}

			if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
				return nil, errors.Wrapf(err, "error creating parent dir from archive for %s", f.Name)
			}

			pf, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return nil, errors.Wrapf(err, "error creating file from archive for %s", f.Name)
			}

			fa, err := f.Open()
			if err != nil {
				return nil, errors.Wrapf(err, "error opening file in archive: %s", f.Name)
			}

			if _, err := io.Copy(pf, fa); err != nil {
				return nil, errors.Wrapf(err, "error extracting file from archive: %s", f.Name)
			}
			pf.Close()
			fa.Close()
		}

		// find blender project
		projectFiles, err := filepath.Glob(filepath.Join(tmpDir, "*.blend"))
		if err != nil {
			return nil, err
		}

		if len(projectFiles) == 0 {
			return nil, errors.New("Blender project file (.blend) not found in zip archive")
		}

		projectFilePath = projectFiles[0]
		if len(projectFiles) > 1 {
			logrus.Warnf("multiple Blender (.blend) projects found; using first detected: %s", projectFilePath)
		}
	default:
	}

	blenderCfg, err := generateBlenderRenderConfig(job)
	if err != nil {
		return nil, err
	}

	blenderConfigPath := filepath.Join(tmpDir, "render.py")

	if err := ioutil.WriteFile(blenderConfigPath, []byte(blenderCfg), 0644); err != nil {
		return nil, err
	}

	// lookup binary path
	// check config first and if not there attempt to resolve from PATH
	blenderBinaryPath := ""
	for _, engine := range w.config.RenderEngines {
		logrus.Debugf("checking render engine config: %s", engine)
		if strings.ToLower(engine.Name) == "blender" {
			logrus.Infof("using blender render path: %s", engine.Path)
			blenderBinaryPath = engine.Path
			break
		}
	}
	if blenderBinaryPath == "" {
		bp, err := exec.LookPath(blenderExecutableName)
		if err != nil {
			return nil, err
		}
		blenderBinaryPath = bp
	}

	workerInfo := w.getWorkerInfo()

	logrus.Debugf("worker info: %+v", workerInfo)

	jobStatus := &api.JobStatus{
		Job:       job,
		Worker:    workerInfo,
		Status:    api.JobStatus_RENDERING,
		StartedAt: time.Now(),
	}

	if err := w.ds.UpdateJobStatus(ctx, jobStatus); err != nil {
		return nil, err
	}

	args := []string{
		"-b",
		"--factory-startup",
		projectFilePath,
		"-P",
		blenderConfigPath,
		"--debug-python",
		"-f",
		fmt.Sprintf("%d", job.RenderFrame),
	}
	args = append(args, blenderCommandPlatformArgs...)

	logrus.Debugf("blender args: %v", args)
	c := exec.CommandContext(ctx, blenderBinaryPath, args...)

	out, err := c.CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, string(out))
	}

	logrus.Debug(string(out))

	// upload results to minio
	files, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		logrus.Debugf("uploading output file %s", f)
		rf, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		defer rf.Close()

		fs, err := rf.Stat()
		if err != nil {
			return nil, err
		}
		if fs.IsDir() {
			continue
		}

		if _, err := mc.PutObject(ctx, w.config.S3Bucket, path.Join(finca.S3RenderPath, job.ID, filepath.Base(f)), rf, fs.Size(), minio.PutObjectOptions{ContentType: "application/octet-stream"}); err != nil {
			return nil, err
		}
	}

	// upload blender log for render
	logFilename := fmt.Sprintf("%s_%04d.log", outputPrefix, job.RenderFrame)
	logFile, err := os.Create(filepath.Join(tmpDir, logFilename))
	if err != nil {
		return nil, err
	}
	defer logFile.Close()

	if _, err := logFile.Write(out); err != nil {
		return nil, err
	}

	if _, err := logFile.Seek(0, 0); err != nil {
		return nil, err
	}

	fi, err := logFile.Stat()
	if err != nil {
		return nil, err
	}

	if _, err := mc.PutObject(ctx, w.config.S3Bucket, path.Join(finca.S3RenderPath, job.ID, logFilename), logFile, fi.Size(), minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		return nil, err
	}

	// check for render slice job; if so, check s3 for number of renders. if matches render slices
	// assume project is complete and composite images into single result
	if job.Request.RenderSlices > 0 {
		if err := w.compositeRender(ctx, job, outputDir); err != nil {
			return nil, err
		}
	}

	jobStatus.Succeeded = true
	jobStatus.Duration = ptypes.DurationProto(time.Now().Sub(start))
	jobStatus.Status = api.JobStatus_FINISHED
	jobStatus.FinishedAt = time.Now()

	if err := w.ds.UpdateJobStatus(ctx, jobStatus); err != nil {
		return nil, err
	}

	return jobStatus, nil
}

func (w *Worker) compositeRender(ctx context.Context, job *api.Job, outputDir string) error {
	mc, err := w.getMinioClient()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("finca-composite-%s", job.ID))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	logrus.Debugf("temp composite dir: %s", tmpDir)
	objCh := mc.ListObjects(ctx, w.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    fmt.Sprintf("%s/%s", finca.S3RenderPath, job.ID),
		Recursive: true,
	})
	slices := []string{}
	for o := range objCh {
		if o.Err != nil {
			return err
		}

		logrus.Debugf("checking key %s", o.Key)
		// filter logs
		if filepath.Ext(o.Key) == ".log" {
			continue
		}
		slices = append(slices, o.Key)
	}

	if int64(len(slices)) < job.Request.RenderSlices {
		logrus.Infof("render for project %s is still in progress; skipping compositing", job.ID)
		return nil
	}

	logrus.Infof("detected complete render; compositing %s", job.ID)
	for _, p := range slices {
		logrus.Debugf("getting object: %s", p)
		obj, err := mc.GetObject(ctx, w.config.S3Bucket, p, minio.GetObjectOptions{})
		if err != nil {
			return err
		}
		fileName := filepath.Base(p)
		logrus.Debugf("creating local %s", fileName)
		f, err := os.Create(filepath.Join(tmpDir, fileName))
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, obj); err != nil {
			return err
		}
		f.Close()
	}

	// process images
	renderStartFrame := job.Request.RenderStartFrame
	renderEndFrame := job.Request.RenderEndFrame
	if renderEndFrame == 0 {
		renderEndFrame = renderStartFrame
	}
	logrus.Debugf("processing %d -> %d frames", renderStartFrame, renderEndFrame)
	for i := renderStartFrame; i <= renderEndFrame; i++ {
		slices := []image.Image{}
		logrus.Debugf("processing frame %d", i)
		files, err := filepath.Glob(filepath.Join(tmpDir, fmt.Sprintf("*_%04d.*", i)))
		if err != nil {
			return err
		}
		for _, f := range files {
			if ext := filepath.Ext(f); strings.ToLower(ext) != ".png" {
				logrus.Warnf("only .png files can be composited; skipping %s", f)
				continue
			}
			logrus.Debugf("adding slice %s", f)
			imgFile, err := os.Open(f)
			if err != nil {
				return err
			}
			img, err := png.Decode(imgFile)
			if err != nil {
				return err
			}
			slices = append(slices, img)
			imgFile.Close()
		}
		if len(slices) == 0 {
			logrus.Warnf("unable to find any render slices for frame %d", i)
			continue
		}

		logrus.Infof("compositing frame %d from %d slices", i, len(slices))
		bounds := slices[0].Bounds()
		finalImage := image.NewRGBA(bounds)
		for _, img := range slices {
			draw.Draw(finalImage, bounds, img, image.ZP, draw.Over)
		}

		finalFilePath := filepath.Join(tmpDir, fmt.Sprintf("%s_%04d.png", job.Request.Name, i))
		logrus.Debugf("creating final composite %s", finalFilePath)
		final, err := os.Create(finalFilePath)
		if err != nil {
			return err
		}
		// TODO: allow for configurable compression levels
		enc := &png.Encoder{
			CompressionLevel: png.NoCompression,
		}
		if err := enc.Encode(final, finalImage); err != nil {
			return err
		}

		st, err := final.Stat()
		if err != nil {
			return err
		}
		final.Close()

		ff, err := os.Open(finalFilePath)
		if err != nil {
			return err
		}
		// sync final composite back to s3
		objectName := path.Join(finca.S3RenderPath, job.ID, fmt.Sprintf("%s_%04d.png", job.Request.Name, i))
		logrus.Infof("uploading composited frame %d (%s) to bucket %s (%d bytes)", i, objectName, w.config.S3Bucket, st.Size())
		if _, err := mc.PutObject(ctx, w.config.S3Bucket, objectName, ff, st.Size(), minio.PutObjectOptions{ContentType: "image/png"}); err != nil {
			return errors.Wrapf(err, "error uploading final composite %s", objectName)
		}
		ff.Close()
	}

	// TODO: remove render slice from s3?

	return nil
}

func generateBlenderRenderConfig(job *api.Job) (string, error) {
	t, err := template.New("blender").Parse(blenderConfigTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, job); err != nil {
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
