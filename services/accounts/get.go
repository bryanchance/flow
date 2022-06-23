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
