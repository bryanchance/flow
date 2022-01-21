package datastore

import (
	"bytes"
	"context"
	"io"
	"path"
	"path/filepath"
	"regexp"

	"git.underland.io/ehazlett/finca"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (d *Datastore) GetLatestRender(ctx context.Context, jobID string) ([]byte, error) {
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
			logrus.Debugf("skipping slice render %s", o.Key)
			continue
		}
		latestRenderPath = o.Key
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
