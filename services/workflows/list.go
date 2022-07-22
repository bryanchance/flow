package workflows

import (
	"context"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
)

func (s *service) ListWorkflows(ctx context.Context, r *api.ListWorkflowsRequest) (*api.ListWorkflowsResponse, error) {
	workflows, err := s.ds.GetWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	return &api.ListWorkflowsResponse{
		Workflows: workflows,
	}, nil
}
