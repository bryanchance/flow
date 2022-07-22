package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/sirupsen/logrus"
)

type Processor struct{}

func (p *Processor) Process(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	w := cfg.Workflow
	logrus.Infof("processed workflow %s", w.ID)
	return &workflows.ProcessorOutput{
		FinishedAt: time.Now(),
		Log:        fmt.Sprintf("processed workflow %s", w.ID),
	}, nil
}
