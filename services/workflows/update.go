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

	api "github.com/fynca/fynca/api/services/workflows/v1"
	ptypes "github.com/gogo/protobuf/types"
)

func (s *service) UpdateWorkflowStatus(ctx context.Context, req *api.UpdateWorkflowStatusRequest) (*ptypes.Empty, error) {
	workflow, err := s.ds.GetWorkflow(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	workflow.Status = req.Status

	if err := s.ds.UpdateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	return empty, nil
}

func (s *service) UpdateWorkflowOutput(ctx context.Context, req *api.UpdateWorkflowOutputRequest) (*ptypes.Empty, error) {
	// TODO
	return empty, nil
}
