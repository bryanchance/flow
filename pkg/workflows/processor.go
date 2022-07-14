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
