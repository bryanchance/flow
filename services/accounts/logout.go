package accounts

import (
	"context"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
	ptypes "github.com/gogo/protobuf/types"
)

func (s *service) Logout(ctx context.Context, req *api.LogoutRequest) (*ptypes.Empty, error) {
	if err := s.authenticator.Logout(ctx); err != nil {
		return nil, err
	}

	return empty, nil
}
