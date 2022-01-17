package datastore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/gogo/protobuf/jsonpb"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
)

type ByCreatedAt []*api.Job

func (s ByCreatedAt) Len() int           { return len(s) }
func (s ByCreatedAt) Less(i, j int) bool { return s[i].CreatedAt.After(s[j].CreatedAt) }
func (s ByCreatedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// GetJobs returns a list of jobs
func (d *Datastore) GetJobs(ctx context.Context) ([]*api.Job, error) {
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    finca.S3ProjectPath,
		Recursive: true,
	})

	found := map[string]struct{}{}
	jobs := []*api.Job{}
	for o := range objCh {
		if o.Err != nil {
			return nil, o.Err
		}

		k := strings.SplitN(o.Key, "/", 2)
		jobID := path.Dir(k[1])

		if _, ok := found[jobID]; ok {
			continue
		}

		found[jobID] = struct{}{}

		jobPath := path.Join(finca.S3ProjectPath, jobID, finca.S3JobPath)
		obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "error getting object from storage %s", jobPath)
		}

		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, obj); err != nil {
			return nil, err
		}

		j := &api.Job{}
		if err := jsonpb.Unmarshal(buf, j); err != nil {
			return nil, err
		}

		jobs = append(jobs, j)
	}
	sort.Sort(ByCreatedAt(jobs))

	return jobs, nil
}

// GetJob returns a job by id from the datastore
func (d *Datastore) GetJob(ctx context.Context, jobID string) (*api.Job, error) {
	jobPath := path.Join(finca.S3ProjectPath, jobID, finca.S3JobPath)
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, jobPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	j := &api.Job{}
	if err := jsonpb.Unmarshal(buf, j); err != nil {
		return nil, err
	}

	return j, nil
}

// UpdateJob updates the status of a job
func (d *Datastore) UpdateJob(ctx context.Context, job *api.Job) error {
	jobPath := getJobPath(job.ID, int(job.RenderSliceIndex))

	j, err := d.GetJob(ctx, job.ID)
	if err != nil {
		if !strings.Contains(err.Error(), "key does not exist") {
			return err
		}
	}

	// update from existing if zero
	if j != nil {
		if job.QueuedAt.IsZero() {
			job.QueuedAt = j.QueuedAt
		}
		if job.StartedAt.IsZero() {
			job.StartedAt = j.StartedAt
		}
		if job.FinishedAt.IsZero() {
			job.FinishedAt = j.FinishedAt
		}

	}

	buf := &bytes.Buffer{}
	m := &jsonpb.Marshaler{}
	if err := m.Marshal(buf, job); err != nil {
		return err
	}

	if _, err := d.storageClient.PutObject(ctx, d.config.S3Bucket, jobPath, buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: finca.S3JobContentType}); err != nil {
		return errors.Wrap(err, "error updating job status")
	}

	return nil
}

// DeleteJob deletes a job and all related files from the datastore
func (d *Datastore) DeleteJob(ctx context.Context, id string) error {
	// delete project
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(finca.S3ProjectPath, id),
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

	// delete renders
	rObjCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    path.Join(finca.S3RenderPath, id),
		Recursive: true,
	})

	for o := range rObjCh {
		if o.Err != nil {
			return o.Err
		}

		if err := d.storageClient.RemoveObject(ctx, d.config.S3Bucket, o.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func getStoragePath(jobID string) string {
	return path.Join(finca.S3ProjectPath, jobID)
}

func getJobPath(jobID string, renderSliceIndex int) string {
	jobPath := path.Join(getStoragePath(jobID), finca.S3JobPath)
	// check for render slice
	if idx := renderSliceIndex; idx > -1 {
		jobPath = path.Join(jobPath) + fmt.Sprintf(".%d", idx)
	}
	return jobPath
}
