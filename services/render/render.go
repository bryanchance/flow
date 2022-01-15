package render

import (
	"context"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
)

func (s *service) GetLatestRender(ctx context.Context, r *api.GetLatestRenderRequest) (*api.GetLatestRenderResponse, error) {
	data, err := s.ds.GetLatestRender(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	return &api.GetLatestRenderResponse{
		ID:   r.ID,
		Data: data,
	}, nil
}
