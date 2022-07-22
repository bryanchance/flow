package workflows

import (
	"context"
	"time"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
)

type ProcessorConfig struct {
	Workflow       *api.Workflow
	InputDir       string
	InputWorkflows []*api.Workflow
}

type ProcessorOutput struct {
	FinishedAt time.Time
	Duration   time.Duration
	Parameters map[string]string
	Log        string
	OutputDir  string
}

type Processor interface {
	Process(context.Context, *ProcessorConfig) (*ProcessorOutput, error)
}
