package worker

import (
	"fmt"
	"strings"
	"testing"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
)

func TestRenderJobTemplate(t *testing.T) {
	j := &api.Job{
		ID: "test-render-job",
		Request: &api.JobRequest{
			Name:             "test",
			ResolutionX:      int64(1920),
			ResolutionY:      int64(1080),
			ResolutionScale:  int64(100),
			RenderSamples:    int64(256),
			RenderStartFrame: int64(1),
			RenderUseGPU:     false,
		},
		OutputDir: "/tmp/render/test-render-job",
	}

	c, err := generateBlenderRenderConfig(j)
	if err != nil {
		t.Fatal(err)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionX)) == -1 {
		t.Errorf("expected resolution x %d", j.Request.ResolutionX)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionY)) == -1 {
		t.Errorf("expected resolution y %d", j.Request.ResolutionY)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionScale)) == -1 {
		t.Errorf("expected resolution scale %d", j.Request.ResolutionScale)
	}

	if strings.IndexAny(c, j.OutputDir) == -1 {
		t.Errorf("expected filepath %q", j.OutputDir)
	}
}

func TestRenderJobTemplateRenderSlices(t *testing.T) {
	j := &api.Job{
		ID: "test-render-job-slices",
		Request: &api.JobRequest{
			Name:             "test-slice",
			ResolutionX:      int64(1920),
			ResolutionY:      int64(1080),
			ResolutionScale:  int64(100),
			RenderSamples:    int64(256),
			RenderStartFrame: int64(1),
			RenderSlices:     int64(10),
			RenderUseGPU:     false,
		},
		RenderSliceMinX: float32(1),
		RenderSliceMaxX: float32(2),
		RenderSliceMinY: float32(1),
		RenderSliceMaxY: float32(2),
		OutputDir:       "/tmp/render/test-render-job-slices",
	}

	c, err := generateBlenderRenderConfig(j)
	if err != nil {
		t.Fatal(err)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionX)) == -1 {
		t.Errorf("expected resolution x %d", j.Request.ResolutionX)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionY)) == -1 {
		t.Errorf("expected resolution y %d", j.Request.ResolutionY)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionScale)) == -1 {
		t.Errorf("expected resolution scale %d", j.Request.ResolutionScale)
	}

	if strings.IndexAny(c, j.OutputDir) == -1 {
		t.Errorf("expected filepath %q", j.OutputDir)
	}
}

func TestRenderJobTemplateGPU(t *testing.T) {
	j := &api.Job{
		ID: "test-render-job-gpu",
		Request: &api.JobRequest{
			Name:             "test",
			ResolutionX:      int64(1920),
			ResolutionY:      int64(1080),
			ResolutionScale:  int64(50),
			RenderSamples:    int64(256),
			RenderStartFrame: int64(1),
			RenderUseGPU:     true,
		},
		OutputDir: "/tmp/render/test-render-job-gpu",
	}

	c, err := generateBlenderRenderConfig(j)
	if err != nil {
		t.Fatal(err)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionX)) == -1 {
		t.Errorf("expected resolution x %d", j.Request.ResolutionX)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionY)) == -1 {
		t.Errorf("expected resolution y %d", j.Request.ResolutionY)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", j.Request.ResolutionScale)) == -1 {
		t.Errorf("expected resolution scale %d", j.Request.ResolutionScale)
	}

	if strings.IndexAny(c, j.OutputDir) == -1 {
		t.Errorf("expected filepath %q", j.OutputDir)
	}
}
