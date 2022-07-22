package accounts

import (
	"context"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
)

func (s *service) Authenticate(ctx context.Context, req *api.AuthenticateRequest) (*api.AuthenticateResponse, error) {
	data, err := s.authenticator.Authenticate(ctx, req.Username, []byte(req.Password))
	if err != nil {
		return nil, err
	}

	acct, err := s.ds.GetAccount(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	return &api.AuthenticateResponse{
		Account: acct,
		Config:  data,
	}, nil
}
