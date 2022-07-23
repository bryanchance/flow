package processor

import (
	"context"

	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	cfg *workflows.Config
}

func NewProcessor(cfg *workflows.Config) (*Processor, error) {
	return &Processor{
		cfg: cfg,
	}, nil
}

func (p *Processor) Process(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	w := cfg.Workflow
	logrus.Debugf("processing workflow %s (%s)", w.Name, w.ID)
	wCtx, cancel := context.WithTimeout(ctx, p.cfg.WorkflowTimeout)
	defer cancel()
	output, err := p.processWorkflow(wCtx, cfg)
	if err != nil {
		return nil, err
	}
	return output, nil
}
