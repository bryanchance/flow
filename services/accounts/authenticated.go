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
