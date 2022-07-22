package workflows

import (
	"context"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
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
