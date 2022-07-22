package accounts

import (
	"context"
	"fmt"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
	ptypes "github.com/gogo/protobuf/types"
)

func (s *service) CreateAccount(ctx context.Context, req *api.CreateAccountRequest) (*ptypes.Empty, error) {
	// check for existing
	if acct, err := s.ds.GetAccount(ctx, req.Account.Username); err == nil && acct != nil {
		return empty, fmt.Errorf("account with username %s already exists", req.Account.Username)
	}

	if err := s.ds.CreateAccount(ctx, req.Account); err != nil {
		return nil, err
	}
	return empty, nil
}
