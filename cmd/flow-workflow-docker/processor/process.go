package processor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/sirupsen/logrus"
)

type containerConfig struct {
	Image string
	Args  []string
}

func (p *Processor) processWorkflow(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	logrus.Debugf("processing workflow %s", cfg.Workflow.ID)
	w := cfg.Workflow
	output := &workflows.ProcessorOutput{
		Parameters: map[string]string{},
	}

	startedAt := time.Now()

	// get params
	pConfig, err := parseParameters(w)
	if err != nil {
		return nil, err
	}

	if pConfig.Image == "" {
		return nil, fmt.Errorf("image must be specified")
	}

	// TODO: run

	cID, err := p.docker.createContainer(ctx, pConfig.Image, pConfig.Args, cfg.InputDir)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("created container %s", cID)

	// wait
	logrus.Debugf("waiting on container %s", cID)
	if err := p.docker.waitContainer(ctx, cID); err != nil {
		return nil, err
	}

	// logs
	logrus.Debugf("getting container logs for %s", cID)
	logBuf := &bytes.Buffer{}
	r, err := p.docker.containerLogs(ctx, cID)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("deleting container %s", cID)
	if err := p.docker.deleteContainer(ctx, cID); err != nil {
		return nil, err
	}

	if _, err := io.Copy(logBuf, r); err != nil {
		return nil, err
	}

	output.Log = string(logBuf.Bytes())

	// workflow output info
	output.Parameters["image"] = pConfig.Image
	output.Parameters["args"] = strings.Join(pConfig.Args, ",")

	output.FinishedAt = time.Now()
	output.Duration = output.FinishedAt.Sub(startedAt)

	logrus.Infof("workflow complete: %s", w.ID)

	return output, nil
}

func parseParameters(w *api.Workflow) (*containerConfig, error) {
	cfg := &containerConfig{}

	for k, v := range w.Parameters {
		switch strings.ToLower(k) {
		case "image":
			cfg.Image = v
		case "args":
			cfg.Args = strings.Fields(v)
		default:
			return nil, fmt.Errorf("unknown parameter specified: %s", k)
		}
	}

	return cfg, nil
}
