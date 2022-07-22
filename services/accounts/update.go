package accounts

import (
	"context"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
	ptypes "github.com/gogo/protobuf/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *service) UpdateAccount(ctx context.Context, req *api.UpdateAccountRequest) (*ptypes.Empty, error) {
	// check if requesting username matches authenticated or admin
	username := ctx.Value(flow.CtxUsernameKey).(string)
	isAdmin := ctx.Value(flow.CtxAdminKey).(bool)
	if username != req.Account.Username && !isAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "access denied")
	}

	account, err := s.ds.GetAccount(ctx, req.Account.Username)
	if err != nil {
		return nil, err
	}

	// only update allowed fields
	// TODO: grpc fieldmask ?

	account.FirstName = req.Account.FirstName
	account.LastName = req.Account.LastName
	// TODO: email verification
	account.Email = req.Account.Email

	if err := s.ds.UpdateAccount(ctx, account); err != nil {
		return nil, err
	}

	return empty, nil
}

func (s *service) ChangePassword(ctx context.Context, req *api.ChangePasswordRequest) (*ptypes.Empty, error) {
	account, err := s.ds.GetAccount(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	// check if requesting username matches authenticated or admin
	username := ctx.Value(flow.CtxUsernameKey).(string)
	isAdmin := ctx.Value(flow.CtxAdminKey).(bool)
	if username != req.Username && !isAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "access denied")
	}

	if err := s.ds.ChangePassword(ctx, account, req.Password); err != nil {
		return nil, err
	}

	return empty, nil
}
