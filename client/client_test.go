package client

import (
	"fmt"
	"path"
	"testing"

	"git.underland.io/ehazlett/finca"
)

func TestGetJobStatusPath(t *testing.T) {
	jobID := "abc"
	renderSliceIndex := -1
	sp := getJobStatusPath(jobID, renderSliceIndex)
	expected := path.Join(finca.S3ProjectPath, jobID, finca.S3JobStatusPath)
	if sp != expected {
		t.Fatalf("expected %s; received %s", expected, sp)
	}
}

func TestGetJobStatusPathRenderSlices(t *testing.T) {
	jobID := "abc"
	renderSliceIndex := 1

	sp := getJobStatusPath(jobID, renderSliceIndex)
	expected := path.Join(finca.S3ProjectPath, jobID, fmt.Sprintf("%s.%d", finca.S3JobStatusPath, renderSliceIndex))
	if sp != expected {
		t.Fatalf("expected %s; received %s", expected, sp)
	}
}
