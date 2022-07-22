package info

import (
	"context"

	api "github.com/ehazlett/flow/api/services/info/v1"
	"github.com/ehazlett/flow/version"
)

func (s *service) Version(ctx context.Context, r *api.VersionRequest) (*api.VersionResponse, error) {
	return &api.VersionResponse{
		Name:          version.Name,
		Version:       version.Version,
		Build:         version.Build,
		Commit:        version.GitCommit,
		Authenticator: s.authenticator.Name(),
	}, nil
}
