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
