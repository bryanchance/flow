package datastore

import (
	"testing"
	"time"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	ptypes "github.com/gogo/protobuf/types"
)

func TestMergeJobsQueued(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_QUEUED,
	}
	s2 := &api.Job{
		Status: api.Job_QUEUED,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_QUEUED {
		t.Fatalf("expected status %s; received %s", api.Job_QUEUED, job.Status)
	}
}

func TestMergeJobsQueuedAndRendering(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_QUEUED,
	}
	s2 := &api.Job{
		Status: api.Job_RENDERING,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_RENDERING {
		t.Fatalf("expected status %s; received %s", api.Job_RENDERING, job.Status)
	}
}

func TestMergeJobsRenderingAndQueued(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_RENDERING,
	}
	s2 := &api.Job{
		Status: api.Job_QUEUED,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_RENDERING {
		t.Fatalf("expected status %s; received %s", api.Job_RENDERING, job.Status)
	}
}

func TestMergeJobsQueuedAndError(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_QUEUED,
	}
	s2 := &api.Job{
		Status: api.Job_ERROR,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_ERROR {
		t.Fatalf("expected status %s; received %s", api.Job_ERROR, job.Status)
	}
}

func TestMergeJobsErrorAndQueued(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_ERROR,
	}
	s2 := &api.Job{
		Status: api.Job_QUEUED,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_ERROR {
		t.Fatalf("expected status %s; received %s", api.Job_ERROR, job.Status)
	}
}

func TestMergeJobsQueuedAndFinished(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_QUEUED,
	}
	s2 := &api.Job{
		Status: api.Job_FINISHED,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_RENDERING {
		t.Fatalf("expected status %s; received %s", api.Job_RENDERING, job.Status)
	}
}

func TestMergeJobsFinishedAndQueued(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_FINISHED,
	}
	s2 := &api.Job{
		Status: api.Job_QUEUED,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_RENDERING {
		t.Fatalf("expected status %s; received %s", api.Job_RENDERING, job.Status)
	}
}

func TestMergeJobsRendering(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_RENDERING,
	}
	s2 := &api.Job{
		Status: api.Job_RENDERING,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_RENDERING {
		t.Fatalf("expected status %s; received %s", api.Job_RENDERING, job.Status)
	}
}

func TestMergeJobsFinished(t *testing.T) {
	s1 := &api.Job{
		Status: api.Job_FINISHED,
	}
	s2 := &api.Job{
		Status: api.Job_FINISHED,
	}
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if job.Status != api.Job_FINISHED {
		t.Fatalf("expected status %s; received %s", api.Job_FINISHED, job.Status)
	}
}

func TestMergeJobsDuration(t *testing.T) {
	s1 := &api.Job{
		Status:   api.Job_FINISHED,
		Duration: ptypes.DurationProto(time.Second * 5),
	}
	s2 := &api.Job{
		Status:   api.Job_FINISHED,
		Duration: ptypes.DurationProto(time.Second * 5),
	}
	expectedDuration := ptypes.DurationProto(time.Second * 10)
	sliceJobs := []*api.Job{s1, s2}
	job := &api.Job{
		SliceJobs: sliceJobs,
	}

	if err := mergeSliceJobs(job); err != nil {
		t.Fatal(err)
	}

	if !job.Duration.Equal(expectedDuration) {
		t.Fatalf("expected duration %s; received %s", expectedDuration, job.Duration)
	}
}
