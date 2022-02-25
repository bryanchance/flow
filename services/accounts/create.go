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
	"fmt"

	api "git.underland.io/ehazlett/fynca/api/services/accounts/v1"
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
