package workflows

import (
	"context"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
)

func (s *service) GetWorkflow(ctx context.Context, r *api.GetWorkflowRequest) (*api.GetWorkflowResponse, error) {
	workflow, err := s.ds.GetWorkflow(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	return &api.GetWorkflowResponse{
		Workflow: workflow,
	}, nil
}
