package workflows

import (
	"context"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
)

func (s *service) ListWorkflows(ctx context.Context, r *api.ListWorkflowsRequest) (*api.ListWorkflowsResponse, error) {
	if len(r.Labels) > 0 {
		ctx = context.WithValue(ctx, flow.CtxDatastoreLabels, r.Labels)
	}
	workflows, err := s.ds.GetWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	return &api.ListWorkflowsResponse{
		Workflows: workflows,
	}, nil
}
