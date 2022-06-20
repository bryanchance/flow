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
