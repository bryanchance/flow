package datastore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// UpdateJobLog updates the specified job log
func (d *Datastore) UpdateJobLog(ctx context.Context, log *api.JobLog) error {
	logPath := getJobLogPath(log.Namespace, log.ID)

	logrus.Debugf("updating job log %s", logPath)

	data := []byte(log.Log)
	buf := bytes.NewBuffer(data)
	if _, err := d.storageClient.PutObject(ctx, d.config.S3Bucket, logPath, buf, int64(len(data)), minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		return err
	}

	return nil
}

// GetRenderLog gets the specified render log for the job
func (d *Datastore) GetRenderLog(ctx context.Context, jobID string, frame int64, slice int64) (*api.RenderLog, error) {
	namespace := ctx.Value(fynca.CtxNamespaceKey).(string)
	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	switch job.Status {
	case api.JobStatus_QUEUED, api.JobStatus_RENDERING:
		return nil, nil
	}

	renderPath := path.Join(namespace, fynca.S3RenderPath, jobID)
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    renderPath,
		Recursive: true,
	})

	logPath := ""
	for o := range objCh {
		if o.Err != nil {
			return nil, o.Err
		}

		// filter logs
		if filepath.Ext(o.Key) != ".log" {
			continue
		}
		// ignore slice renders
		if slice > -1 {
			sliceMatch, err := regexp.MatchString(fmt.Sprintf(".*_slice_%d_%04d.log", slice, frame), o.Key)
			if err != nil {
				return nil, err
			}
			if sliceMatch {
				logPath = o.Key
				break
			}
			continue
		}
		frameMatch, err := regexp.MatchString(fmt.Sprintf(".*_%04d.log", frame), o.Key)
		if err != nil {
			return nil, err
		}
		if frameMatch {
			logPath = o.Key
			break
		}
	}

	if logPath == "" {
		return nil, nil
	}

	// get key data
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, logPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "error getting log from storage %s", logPath)
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	return &api.RenderLog{
		Log:   string(buf.Bytes()),
		Frame: frame,
		Slice: slice,
	}, nil
}
