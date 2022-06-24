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

	api "github.com/ehazlett/flow/api/services/accounts/v1"
	ptypes "github.com/gogo/protobuf/types"
)

// Authenticated returns nil indicating that the authenticated request succeeded
// if the requestor is not authenticated the auth middleware will intercept and
// return a GRPC Unauthenticated error
func (s *service) Authenticated(_ context.Context, _ *api.AuthenticatedRequest) (*ptypes.Empty, error) {
	return empty, nil
}
