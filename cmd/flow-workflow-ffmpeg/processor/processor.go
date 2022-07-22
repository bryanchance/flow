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
	ffmpegPath string
	cfg        *workflows.Config
}

func NewProcessor(ffmpegPath string, cfg *workflows.Config) (*Processor, error) {
	if ffmpegPath == "" {
		// resolve in path
		fp, err := exec.LookPath(ffmpegExecutableName)
		if err != nil {
			return nil, err
		}
		ffmpegPath = fp
	}
	if _, err := os.Stat(ffmpegPath); err != nil {
		return nil, errors.Wrapf(err, "cannot find ffmpeg at path %q", ffmpegPath)
	}

	return &Processor{
		ffmpegPath: ffmpegPath,
		cfg:        cfg,
	}, nil
}

func (p *Processor) Process(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	w := cfg.Workflow
	logrus.Debugf("processing workflow %s (%s)", w.Name, w.ID)
	wCtx, cancel := context.WithTimeout(ctx, p.cfg.WorkflowTimeout)
	defer cancel()
	output, err := p.encodeWorkflow(wCtx, cfg)
	if err != nil {
		return nil, err
	}
	return output, nil
}
