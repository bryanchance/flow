package render

import (
	"context"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
)

func (s *service) JobLog(ctx context.Context, r *api.JobLogRequest) (*api.JobLogResponse, error) {
	log, err := s.ds.GetJobLog(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	return &api.JobLogResponse{
		JobLog: log,
	}, nil
}

func (s *service) RenderLog(ctx context.Context, r *api.RenderLogRequest) (*api.RenderLogResponse, error) {
	log, err := s.ds.GetRenderLog(ctx, r.ID, r.Frame, r.Slice)
	if err != nil {
		return nil, err
	}

	return &api.RenderLogResponse{
		RenderLog: log,
	}, nil
}
