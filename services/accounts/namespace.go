package accounts

import (
	"context"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
)

func (s *service) ListNamespaces(ctx context.Context, req *api.ListNamespacesRequest) (*api.ListNamespacesResponse, error) {
	namespaces, err := s.ds.GetNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	return &api.ListNamespacesResponse{
		Namespaces: namespaces,
	}, nil
}

func (s *service) CreateNamespace(ctx context.Context, req *api.CreateNamespaceRequest) (*api.CreateNamespaceResponse, error) {
	nsID, err := s.ds.CreateNamespace(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}
	return &api.CreateNamespaceResponse{
		ID: nsID,
	}, nil
}

func (s *service) GetNamespace(ctx context.Context, req *api.GetNamespaceRequest) (*api.GetNamespaceResponse, error) {
	ns, err := s.ds.GetNamespace(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return &api.GetNamespaceResponse{
		Namespace: ns,
	}, nil
}
