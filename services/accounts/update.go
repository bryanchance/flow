// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
