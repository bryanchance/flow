package worker

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"git.underland.io/ehazlett/finca"
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
    scene.render.filepath = os.path.join("{{ .OutputDir }}/{{ .Request.Name }}_{{ if gt .Request.RenderSlices 0 }}{{ .RenderSliceMinX }}_{{ .RenderSliceMaxX }}_{{ .RenderSliceMinY }}_{{ .RenderSliceMaxY }}_{{ end }}")
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
	logrus.Infof("processing job %s (%s)", job.ID, job.JobSource)

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
	logrus.Debugf("blender args: %v", args)
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

		if _, err := mc.PutObject(ctx, w.config.S3Bucket, path.Join(finca.S3RenderPath, job.ID, filepath.Base(f)), rf, fs.Size(), minio.PutObjectOptions{ContentType: "application/octet-stream"}); err != nil {
			return err
		}
	}

	// check for render slice job; if so, check s3 for number of renders. if matches render slices
	// assume project is complete and composite images into single result
	if job.Request.RenderSlices > 0 {
		if err := w.compositeRender(ctx, job); err != nil {
			return err
		}
	}

	return nil
}

func (w *Worker) compositeRender(ctx context.Context, job *api.Job) error {
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
