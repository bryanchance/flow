package accounts

import (
	"context"

	api "git.underland.io/ehazlett/fynca/api/services/accounts/v1"
	ptypes "github.com/gogo/protobuf/types"
)

func (s *service) DeleteAccount(ctx context.Context, req *api.DeleteAccountRequest) (*ptypes.Empty, error) {
	return empty, nil
}
