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
	"strings"

	api "github.com/fynca/fynca/api/services/workflows/v1"
	ptypes "github.com/gogo/protobuf/types"
)

func (s *service) DeleteWorkflow(ctx context.Context, r *api.DeleteWorkflowRequest) (*ptypes.Empty, error) {
	workflow, err := s.ds.GetWorkflow(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	js, err := s.natsClient.JetStream()
	if err != nil {
		return empty, err
	}

	if workflow.SequenceID != 0 {
		subj := getWorkflowSubject(workflow)
		if err := js.DeleteMsg(subj, workflow.SequenceID); err != nil {
			// ignore missing
			if !strings.Contains(err.Error(), "no message found") {
				return empty, err
			}
		}
	}

	return empty, nil
}
