package processor

import (
	"context"
	"os"
	"os/exec"

	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	blenderPath string
	cfg         *workflows.Config
}

func NewProcessor(blenderPath string, pCfg *workflows.Config) (*Processor, error) {
	if blenderPath == "" {
		// resolve in path
		bp, err := exec.LookPath(blenderExecutableName)
		if err != nil {
			return nil, err
		}
		blenderPath = bp
	}
	if _, err := os.Stat(blenderPath); err != nil {
		return nil, errors.Wrapf(err, "cannot find blender at path %q", blenderPath)
	}

	return &Processor{
		blenderPath: blenderPath,
		cfg:         pCfg,
	}, nil
}

func (p *Processor) Process(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	w := cfg.Workflow
	logrus.Debugf("processing workflow %s (%s)", w.Name, w.ID)
	wCtx, cancel := context.WithTimeout(ctx, p.cfg.WorkflowTimeout)
	defer cancel()
	output, err := p.renderWorkflow(wCtx, cfg)
	if err != nil {
		return nil, err
	}
	return output, nil
}
