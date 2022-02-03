package render

import (
	"context"

	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	"git.underland.io/ehazlett/fynca/version"
)

func (s *service) Version(ctx context.Context, req *api.VersionRequest) (*api.VersionResponse, error) {
	return &api.VersionResponse{
		Name:          version.Name,
		Version:       version.Version,
		Build:         version.Build,
		Commit:        version.GitCommit,
		Authenticator: s.authenticator.Name(),
	}, nil
}
