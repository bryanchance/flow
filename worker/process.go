package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
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
prop.compute_device_type = 'OPTIX'
# configure gpu
for device in prop.devices:
    if device.type == 'OPTIX':
        device.use = True
{{ end }}

for scene in bpy.data.scenes:
    scene.cycles.device = '{{ if .Request.RenderUseGPU }}GPU{{ else }}CPU{{ end }}'
    scene.cycles.samples = {{ .Request.RenderSamples }}
    scene.render.resolution_x = {{ .Request.ResolutionX }}
    scene.render.resolution_y = {{ .Request.ResolutionY }}
    scene.render.resolution_percentage = {{ .Request.ResolutionScale }}
    scene.render.filepath = os.path.join("{{ .OutputDir }}/{{ .Request.Name }}{{ if gt .Request.RenderSlices 0 }}_{{ .RenderSliceMinX }}_{{ .RenderSliceMaxX }}_{{ .RenderSliceMinY }}_{{ .RenderSliceMaxY }}{{ end }}")
    {{ if gt .Request.RenderSlices 0 }}
    scene.render.border_min_x = {{ .RenderSliceMinX }}
    scene.render.border_max_x = {{ .RenderSliceMaxX }}
    scene.render.border_min_y = {{ .RenderSliceMinY }}
    scene.render.border_max_y = {{ .RenderSliceMaxY }}
    scene.render.use_border = True
    {{ end }}
`
)

func (w *Worker) processJob(ctx context.Context, job *api.Job) error {
	logrus.Debugf("processing job %s (%s)", job.ID, job.JobSource)

	// render with blender
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("finca-%s", job.ID))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	logrus.Debugf("temp work dir: %s", tmpDir)

	// setup render dir
	outputDir := filepath.Join(w.config.JobOutputDir, job.ID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	job.OutputDir = outputDir

	// fetch project source file
	mc, err := w.getMinioClient()
	if err != nil {
		return err
	}

	object, err := mc.GetObject(ctx, w.config.S3Bucket, job.JobSource, minio.GetObjectOptions{})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, job.JobSource)), 0755); err != nil {
		return err
	}

	projectFile, err := os.Create(filepath.Join(tmpDir, job.JobSource))
	if err != nil {
		return err
	}

	if _, err := io.Copy(projectFile, object); err != nil {
		return err
	}

	projectFilePath := projectFile.Name()

	blenderCfg, err := generateBlenderRenderConfig(job)
	if err != nil {
		return err
	}

	blenderConfigPath := filepath.Join(tmpDir, "render.py")

	if err := ioutil.WriteFile(blenderConfigPath, []byte(blenderCfg), 0644); err != nil {
		return err
	}

	blenderBinaryPath, err := exec.LookPath("blender")
	if err != nil {
		return err
	}
	args := []string{
		"-b",
		projectFilePath,
		"-P",
		blenderConfigPath,
		"-f",
		fmt.Sprintf("%d", job.RenderFrame),
	}
	c := exec.CommandContext(ctx, blenderBinaryPath, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}

	logrus.Debug(string(out))

	// upload results to minio
	files, err := filepath.Glob(path.Join(outputDir, "*"))
	if err != nil {
		return err
	}
	logrus.Debug(files)
	for _, f := range files {
		logrus.Debugf(f)
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

		if _, err := mc.PutObject(ctx, w.config.S3Bucket, path.Join("render", job.ID, filepath.Base(f)), rf, fs.Size(), minio.PutObjectOptions{ContentType: "application/octet-stream"}); err != nil {
			return err
		}
	}

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
