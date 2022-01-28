package datastore

import (
	"bytes"
	"context"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	minio "github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

// UpdateJobLog updates the specified job log
func (d *Datastore) UpdateJobLog(ctx context.Context, log *api.JobLog) error {
	logPath := getJobLogPath(log.ID)
	logrus.Debugf("updating job log %s", logPath)
	data := []byte(log.Log)
	buf := bytes.NewBuffer(data)
	if _, err := d.storageClient.PutObject(ctx, d.config.S3Bucket, logPath, buf, int64(len(data)), minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		return err
	}

	return nil
}
