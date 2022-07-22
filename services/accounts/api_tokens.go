package accounts

import (
	"context"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
)

func (s *service) GenerateAPIToken(ctx context.Context, req *api.GenerateAPITokenRequest) (*api.GenerateAPITokenResponse, error) {
	t, err := s.authenticator.GenerateAPIToken(ctx, req.Description)
	if err != nil {
		return nil, err
	}
	return &api.GenerateAPITokenResponse{
		APIToken: t,
	}, nil
}
