// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package processor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type trivyConfig struct {
	Action string
	Target string
}

func (p *Processor) processWorkflow(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	logrus.Debugf("processing workflow %s", cfg.Workflow.ID)
	w := cfg.Workflow
	output := &workflows.ProcessorOutput{
		Parameters: map[string]string{},
	}
	workflowDir := cfg.InputDir

	// setup output dir
	outputDir, err := os.MkdirTemp("", fmt.Sprintf("flow-output-%s", w.ID))
	if err != nil {
		return nil, err
	}
	// set output dir for upload
	output.OutputDir = outputDir

	startedAt := time.Now()

	trivyCfg, err := parseParameters(w)
	if err != nil {
		return nil, err
	}

	outputFilename := filepath.Join(outputDir, "trivy.log")

	if trivyCfg.Action == "" {
		return nil, fmt.Errorf("action must be specified")
	}

	if trivyCfg.Target == "" {
		logrus.Infof("target not specified; using workflow input dir")
		// TODO: check for archive (zip, tar.gz, etc) and extract
		trivyCfg.Target = cfg.InputDir
	}

	// workflow output info
	output.Parameters["action"] = trivyCfg.Action
	output.Parameters["target"] = trivyCfg.Target

	args := []string{
		trivyCfg.Action,
		"--no-progress",
		fmt.Sprintf("--output=%s", outputFilename),
		trivyCfg.Target,
	}

	logrus.Debugf("trivy args: %v", args)
	c := exec.CommandContext(ctx, p.trivyPath, args...)
	c.Dir = workflowDir

	out, err := c.CombinedOutput()
	if err != nil {
		// copy log to tmp
		tmpLogPath := filepath.Join(outputDir, "error.log")
		if err := os.WriteFile(tmpLogPath, out, 0644); err != nil {
			logrus.WithError(err).Errorf("unable to save error log file for workflow %s", w.ID)
		}
		logrus.WithError(err).Errorf("error log available at %s", tmpLogPath)
		return nil, errors.Wrap(err, string(out))
	}
	logrus.Debugf("out: %s", out)
	output.FinishedAt = time.Now()
	output.Duration = output.FinishedAt.Sub(startedAt)
	output.Log = string(out)

	logrus.Debugf("workflow complete: %+v", output)

	return output, nil
}

func parseParameters(w *api.Workflow) (*trivyConfig, error) {
	cfg := &trivyConfig{}

	for k, v := range w.Parameters {
		switch strings.ToLower(k) {
		case "action":
			cfg.Action = v
		case "target":
			cfg.Target = v
		default:
			return nil, fmt.Errorf("unknown parameter specified: %s", k)
		}
	}

	return cfg, nil
}
