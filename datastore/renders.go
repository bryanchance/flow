package datastore

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
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (d *Datastore) GetLatestRender(ctx context.Context, jobID string, frame int64) ([]byte, error) {
	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// check for rendering and if slices return composite
	if job.Status == api.JobStatus_RENDERING {
		// check slices
		for _, frameJob := range job.FrameJobs {
			if frameJob.RenderFrame == frame && len(frameJob.SliceJobs) > 0 {
				return d.GetCompositeRender(ctx, jobID, frame)
			}
		}
	}

	renderPath := path.Join(finca.S3RenderPath, jobID)
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    renderPath,
		Recursive: true,
	})

	latestRenderPath := ""
	for o := range objCh {
		if o.Err != nil {
			return nil, o.Err
		}

		// TODO: support other file types
		if filepath.Ext(o.Key) != ".png" {
			continue
		}
		// ignore slice renders
		sliceMatch, err := regexp.MatchString(".*_slice-\\d+_\\d+\\.[png]", o.Key)
		if err != nil {
			return nil, err
		}
		if sliceMatch {
			continue
		}
		frameMatch, err := regexp.MatchString(fmt.Sprintf(".*_%04d.[png]", frame), o.Key)
		if err != nil {
			return nil, err
		}
		if frameMatch {
			latestRenderPath = o.Key
			break
		}
	}

	if latestRenderPath == "" {
		return nil, nil
	}

	// get key data
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, latestRenderPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "error getting latest render from storage %s", latestRenderPath)
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (d *Datastore) DeleteRenders(ctx context.Context, id string) error {
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(finca.S3RenderPath, id),
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

	return nil
}

func (d *Datastore) GetCompositeRender(ctx context.Context, jobID string, frame int64) ([]byte, error) {
	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting job %s for compositing", jobID)
	}
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("finca-composite-%s", job.ID))
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    fmt.Sprintf("%s/%s", finca.S3RenderPath, job.ID),
		Recursive: true,
	})
	slices := []string{}
	for o := range objCh {
		if o.Err != nil {
			return nil, err
		}

		// filter logs
		if filepath.Ext(o.Key) == ".log" {
			continue
		}

		// filter frame
		frameMatch, err := regexp.MatchString(fmt.Sprintf(".*_%04d.[png]", frame), o.Key)
		if err != nil {
			return nil, err
		}
		if !frameMatch {
			continue
		}
		slices = append(slices, o.Key)
	}

	for _, p := range slices {
		obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, p, minio.GetObjectOptions{})
		if err != nil {
			return nil, err
		}
		fileName := filepath.Base(p)
		f, err := os.Create(filepath.Join(tmpDir, fileName))
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(f, obj); err != nil {
			return nil, err
		}
		f.Close()
	}

	// process images
	frameSlices := []image.Image{}
	files, err := filepath.Glob(filepath.Join(tmpDir, fmt.Sprintf("*_%04d.*", frame)))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if ext := filepath.Ext(f); strings.ToLower(ext) != ".png" {
			logrus.Warnf("only .png files can be composited; skipping %s", f)
			continue
		}
		imgFile, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(imgFile)
		if err != nil {
			return nil, err
		}
		frameSlices = append(frameSlices, img)
		imgFile.Close()
	}
	if len(frameSlices) == 0 {
		return nil, fmt.Errorf("unable to find any render slices for frame %d", frame)
	}

	bounds := frameSlices[0].Bounds()
	finalImage := image.NewRGBA(bounds)
	for _, img := range frameSlices {
		draw.Draw(finalImage, bounds, img, image.ZP, draw.Over)
	}

	finalFilePath := filepath.Join(tmpDir, fmt.Sprintf("%s_%04d.png", job.Request.Name, frame))
	final, err := os.Create(finalFilePath)
	if err != nil {
		return nil, err
	}
	// TODO: allow for configurable compression levels
	enc := &png.Encoder{
		CompressionLevel: png.NoCompression,
	}
	if err := enc.Encode(final, finalImage); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(finalFilePath)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(data).Bytes(), nil
}
