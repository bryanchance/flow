package accounts

import (
	"context"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
	ptypes "github.com/gogo/protobuf/types"
)

func (s *service) DeleteAccount(ctx context.Context, req *api.DeleteAccountRequest) (*ptypes.Empty, error) {
	// TODO: delete account

	// TODO: delete associated api tokens
	return empty, nil
}
