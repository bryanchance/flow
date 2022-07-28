package info

import (
	"context"

	api "github.com/ehazlett/flow/api/services/info/v1"
)

func (s *service) WorkflowInfo(ctx context.Context, r *api.WorkflowInfoRequest) (*api.WorkflowInfoResponse, error) {
	totalWorkflowCount, err := s.ds.GetTotalWorkflowsCount(ctx)
	if err != nil {
		return nil, err
	}
	pendingWorkflowCount, err := s.ds.GetPendingWorkflowsCount(ctx)
	if err != nil {
		return nil, err
	}
	return &api.WorkflowInfoResponse{
		TotalWorkflows:   totalWorkflowCount,
		PendingWorkflows: pendingWorkflowCount,
	}, nil
}
