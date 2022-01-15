package render

import (
	"fmt"
	"path"
	"testing"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
)

func TestGetJobStatusPath(t *testing.T) {
	jobID := "abc"
	s1 := &api.JobStatus{
		Job: &api.Job{
			ID:               jobID,
			RenderSliceIndex: -1,
		},
	}

	sp := getJobStatusPath(s1)
	expected := path.Join(finca.S3ProjectPath, jobID, finca.S3JobStatusPath)
	if sp != expected {
		t.Fatalf("expected %s; received %s", expected, sp)
	}
}

func TestGetJobStatusPathRenderSlices(t *testing.T) {
	jobID := "abc"
	s1 := &api.JobStatus{
		Job: &api.Job{
			ID:               jobID,
			RenderSliceIndex: 1,
		},
	}

	sp := getJobStatusPath(s1)
	expected := path.Join(finca.S3ProjectPath, jobID, fmt.Sprintf("%s.%d", finca.S3JobStatusPath, s1.Job.RenderSliceIndex))
	if sp != expected {
		t.Fatalf("expected %s; received %s", expected, sp)
	}
}
