package accounts

import (
	"context"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
)

func (s *service) GenerateServiceToken(ctx context.Context, req *api.GenerateServiceTokenRequest) (*api.GenerateServiceTokenResponse, error) {
	st, err := s.authenticator.GenerateServiceToken(ctx, req.Description, req.TTL)
	if err != nil {
		return nil, err
	}
	return &api.GenerateServiceTokenResponse{
		ServiceToken: st,
	}, nil
}

func (s *service) ListServiceTokens(ctx context.Context, req *api.ListServiceTokensRequest) (*api.ListServiceTokensResponse, error) {
	tokens, err := s.authenticator.ListServiceTokens(ctx)
	if err != nil {
		return nil, err
	}
	return &api.ListServiceTokensResponse{
		ServiceTokens: tokens,
	}, nil
}
