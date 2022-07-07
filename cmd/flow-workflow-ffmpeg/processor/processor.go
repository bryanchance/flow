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
