package worker

import (
	"fmt"
	"strings"
	"testing"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
)

func TestRenderJobTemplate(t *testing.T) {
	cfg := &blenderConfig{
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
	c, err := generateBlenderRenderConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionX)) == -1 {
		t.Errorf("expected resolution x %d", cfg.Request.ResolutionX)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionY)) == -1 {
		t.Errorf("expected resolution y %d", cfg.Request.ResolutionY)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionScale)) == -1 {
		t.Errorf("expected resolution scale %d", cfg.Request.ResolutionScale)
	}

	if strings.IndexAny(c, cfg.OutputDir) == -1 {
		t.Errorf("expected filepath %q", cfg.OutputDir)
	}
}

func TestRenderJobTemplateRenderSlices(t *testing.T) {
	cfg := &blenderConfig{
		Request: &api.JobRequest{
			Name:             "test",
			ResolutionX:      int64(1920),
			ResolutionY:      int64(1080),
			ResolutionScale:  int64(100),
			RenderSamples:    int64(256),
			RenderStartFrame: int64(1),
			RenderUseGPU:     false,
		},
		SliceJob: &api.SliceJob{
			RenderSliceMinX: float32(0.0),
			RenderSliceMaxX: float32(2.0),
			RenderSliceMinY: float32(1.0),
			RenderSliceMaxY: float32(2.0),
		},
		OutputDir: "/tmp/render/test-render-job",
	}
	c, err := generateBlenderRenderConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionX)) == -1 {
		t.Errorf("expected resolution x %d", cfg.Request.ResolutionX)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionY)) == -1 {
		t.Errorf("expected resolution y %d", cfg.Request.ResolutionY)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionScale)) == -1 {
		t.Errorf("expected resolution scale %d", cfg.Request.ResolutionScale)
	}

	if strings.IndexAny(c, cfg.OutputDir) == -1 {
		t.Errorf("expected filepath %q", cfg.OutputDir)
	}

	if strings.IndexAny(c, fmt.Sprintf("%.1f", cfg.SliceJob.RenderSliceMinX)) == -1 {
		t.Errorf("expected render slice minX %v", cfg.SliceJob.RenderSliceMinX)
	}
}

func TestRenderJobTemplateGPU(t *testing.T) {
	cfg := &blenderConfig{
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

	c, err := generateBlenderRenderConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionX)) == -1 {
		t.Errorf("expected resolution x %d", cfg.Request.ResolutionX)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionY)) == -1 {
		t.Errorf("expected resolution y %d", cfg.Request.ResolutionY)
	}

	if strings.IndexAny(c, fmt.Sprintf("%d", cfg.Request.ResolutionScale)) == -1 {
		t.Errorf("expected resolution scale %d", cfg.Request.ResolutionScale)
	}

	if strings.IndexAny(c, cfg.OutputDir) == -1 {
		t.Errorf("expected filepath %q", cfg.OutputDir)
	}
}
