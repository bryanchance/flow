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
package info

import (
	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/info/v1"
	"github.com/ehazlett/flow/pkg/auth"
	"github.com/ehazlett/flow/services"
	ptypes "github.com/gogo/protobuf/types"
	"google.golang.org/grpc"
)

var (
	empty = &ptypes.Empty{}
)

type service struct {
	config        *flow.Config
	authenticator auth.Authenticator
}

func New(cfg *flow.Config) (services.Service, error) {
	return &service{
		config: cfg,
	}, nil
}

func (s *service) Configure(a auth.Authenticator) error {
	s.authenticator = a
	return nil
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterInfoServer(server, s)
	return nil
}

func (s *service) Type() services.Type {
	return services.InfoService
}

func (s *service) Requires() []services.Type {
	return nil
}

func (s *service) Start() error {
	return nil
}

func (s *service) Stop() error {
	return nil
}
