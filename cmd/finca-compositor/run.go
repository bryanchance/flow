package main

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	nomadapi "github.com/hashicorp/nomad/api"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func runAction(clix *cli.Context) error {
	nc, err := getNomadClient(clix)
	if err != nil {
		return errors.Wrap(err, "error getting nomad client")
	}

	mc, err := getMinioClient(clix)
	if err != nil {
		return err
	}

	// query nomad to wait for jobs
	jobID := clix.String("nomad-job-id")
	region := clix.String("nomad-region")
	namespace := clix.String("nomad-namespace")
	projectName := clix.String("project-name")
	renderSlices := clix.Int("render-slices")

	logrus.Infof("waiting for nomad render tasks to complete in %s (%s)", namespace, region)
	t := time.NewTicker(1 * time.Second)
	for range t.C {
		allocations, _, err := nc.Jobs().Allocations(jobID, false, &nomadapi.QueryOptions{
			Region:    region,
			Namespace: namespace,
		})
		if err != nil {
			return err
		}

		if len(allocations) <= renderSlices {
			logrus.Debugf("not all render tasks have been queueud; waiting")
			continue
		}
		complete := true
		for _, alloc := range allocations {
			if strings.Contains(alloc.Name, fmt.Sprintf("%s-composite", projectName)) {
				continue
			}
			logrus.Debugf("allocation: %s (%s)", alloc.Name, alloc.ClientStatus)
			switch alloc.ClientStatus {
			case nomadapi.AllocClientStatusRunning, nomadapi.AllocClientStatusPending:
				complete = false
				break
			}
		}
		if complete {
			break
		}
	}

	// sync render slices from s3
	tmpDir, err := os.MkdirTemp("", jobID)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	s3Endpoint := clix.String("s3-endpoint")
	s3Bucket := clix.String("s3-bucket")
	s3RenderDirectory := clix.String("s3-render-directory")
	logrus.Infof("syncing render slices from %s/%s", s3Endpoint, s3Bucket)

	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()

	// get objects
	logrus.Debugf("getting objects from %s/%s", s3RenderDirectory, jobID)
	objCh := mc.ListObjects(ctx, s3Bucket, minio.ListObjectsOptions{
		Prefix:    fmt.Sprintf("%s/%s", s3RenderDirectory, jobID),
		Recursive: true,
	})
	for o := range objCh {
		if o.Err != nil {
			return err
		}
		logrus.Debugf("getting object: %s", o.Key)
		obj, err := mc.GetObject(ctx, s3Bucket, o.Key, minio.GetObjectOptions{})
		if err != nil {
			return err
		}
		fileName := filepath.Base(o.Key)
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
	renderStartFrame := clix.Int("render-start-frame")
	renderEndFrame := clix.Int("render-end-frame")
	if renderEndFrame == 0 {
		renderEndFrame = renderStartFrame
	}
	logrus.Debugf("processing %d -> %d frames", renderStartFrame, renderEndFrame)
	for i := renderStartFrame; i <= renderEndFrame; i++ {
		slices := []image.Image{}
		logrus.Debugf("processing frame %d", i)
		files, err := filepath.Glob(filepath.Join(tmpDir, fmt.Sprintf("render_%d_*", i)))
		if err != nil {
			return err
		}
		for _, f := range files {
			if ext := filepath.Ext(f); strings.ToLower(ext) != ".png" {
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

		finalFilePath := filepath.Join(tmpDir, fmt.Sprintf("%s_%d.png", projectName, i))
		logrus.Debugf("creating final composite %s", finalFilePath)
		final, err := os.Create(finalFilePath)
		if err != nil {
			return err
		}
		if err := png.Encode(final, finalImage); err != nil {
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
		objectName := path.Join(s3RenderDirectory, jobID, fmt.Sprintf("%s_%04d.png", projectName, i))
		logrus.Infof("uploading composited frame %d (%s) to bucket %s (%d bytes)", i, objectName, s3Bucket, st.Size())
		if _, err := mc.PutObject(ctx, s3Bucket, objectName, ff, st.Size(), minio.PutObjectOptions{ContentType: "image/png"}); err != nil {
			return errors.Wrapf(err, "error uploading final composite %s", objectName)
		}
		ff.Close()

		// TODO: remove render slice from s3?
	}

	return nil
}
