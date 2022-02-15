package render

import (
	"context"

	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	"git.underland.io/ehazlett/fynca/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
)

func (s *service) GetLatestRender(ctx context.Context, req *api.GetLatestRenderRequest) (*api.GetLatestRenderResponse, error) {
	var span trace.Span
	ctx, span = tracing.StartSpan(ctx, "GetLatestRender")
	defer span.End()

	url, err := s.ds.GetLatestRender(ctx, req.ID, req.Frame, req.TTL)
	if err != nil {
		return nil, err
	}

	return &api.GetLatestRenderResponse{
		Url:   url,
		Frame: req.Frame,
	}, nil
}
