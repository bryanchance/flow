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

type ByCreatedAt []*api.JobStatus

func (s ByCreatedAt) Len() int           { return len(s) }
func (s ByCreatedAt) Less(i, j int) bool { return s[i].Job.CreatedAt.After(s[j].Job.CreatedAt) }
func (s ByCreatedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// GetJobs returns a list of jobs
func (d *Datastore) GetJobs(ctx context.Context) ([]*api.JobStatus, error) {
	objCh := d.storageClient.ListObjects(ctx, d.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    finca.S3ProjectPath,
		Recursive: true,
	})

	found := map[string]struct{}{}
	jobs := []*api.JobStatus{}
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

		statusPath := path.Join(finca.S3ProjectPath, jobID, finca.S3JobStatusPath)
		obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, statusPath, minio.GetObjectOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "error getting object from storage %s", statusPath)
		}

		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, obj); err != nil {
			return nil, err
		}

		j := &api.JobStatus{}
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
	statusPath := path.Join(finca.S3ProjectPath, jobID, finca.S3JobStatusPath)
	obj, err := d.storageClient.GetObject(ctx, d.config.S3Bucket, statusPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, err
	}

	j := &api.JobStatus{}
	if err := jsonpb.Unmarshal(buf, j); err != nil {
		return nil, err
	}

	return j.Job, nil
}

// UpdateJobStatus updates the status of a job
func (d *Datastore) UpdateJobStatus(ctx context.Context, js *api.JobStatus) error {
	statusPath := getJobStatusPath(js.Job.ID, int(js.Job.RenderSliceIndex))

	buf := &bytes.Buffer{}
	m := &jsonpb.Marshaler{}
	if err := m.Marshal(buf, js); err != nil {
		return err
	}

	if _, err := d.storageClient.PutObject(ctx, d.config.S3Bucket, statusPath, buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: finca.S3JobStatusContentType}); err != nil {
		return errors.Wrap(err, "error updating job status")
	}

	return nil
}

// DeleteJob deletes a job and all related files from the datastore
func (d *Datastore) DeleteJob(ctx context.Context, id string) error {
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

	return nil
}

func getStoragePath(jobID string) string {
	return path.Join(finca.S3ProjectPath, jobID)
}

func getJobStatusPath(jobID string, renderSliceIndex int) string {
	statusPath := path.Join(getStoragePath(jobID), finca.S3JobStatusPath)
	// check for render slice
	if idx := renderSliceIndex; idx > -1 {
		statusPath = path.Join(statusPath) + fmt.Sprintf(".%d", idx)
	}
	return statusPath
}
