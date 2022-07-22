package accounts

import (
	"context"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
)

// GetAccount returns the requested account from the datastore
func (s *service) GetAccount(ctx context.Context, req *api.GetAccountRequest) (*api.GetAccountResponse, error) {
	account, err := s.ds.GetAccount(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	return &api.GetAccountResponse{
		Account: account,
	}, nil
}

// GetAccountProfile returns the account for the current user in the context
func (s *service) GetAccountProfile(ctx context.Context, req *api.GetAccountProfileRequest) (*api.GetAccountProfileResponse, error) {
	username := ctx.Value(flow.CtxUsernameKey).(string)
	account, err := s.ds.GetAccount(ctx, username)
	if err != nil {
		return nil, err
	}
	return &api.GetAccountProfileResponse{
		Account: account,
	}, nil
}
