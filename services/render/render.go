package render

import (
	"context"

	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
)

func (s *service) GetLatestRender(ctx context.Context, req *api.GetLatestRenderRequest) (*api.GetLatestRenderResponse, error) {
	job, err := s.ds.GetJob(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	url, err := s.ds.GetLatestRender(ctx, req.ID, job.Request.Name, req.Frame, job.Request.RenderSlices > 0, req.TTL)
	if err != nil {
		return nil, err
	}

	return &api.GetLatestRenderResponse{
		Url:   url,
		Frame: req.Frame,
	}, nil
}
