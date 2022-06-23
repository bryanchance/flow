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
package auth

import (
	"context"
	"time"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
	"google.golang.org/grpc"
)

const (
	// AuthenticateMethod is the GRPC method that is used for client authentication
	AuthenticateMethod = "/flow.services.accounts.v1.Accounts/Authenticate"
)

type Authenticator interface {
	// Name returns the name of the authenticator
	Name() string
	// GetAccount gets a Flow account from the authenticator
	GetAccount(ctx context.Context, username string) (*api.Account, error)
	// Authenticate authenticates the specified user and returns a byte array from the provider
	Authenticate(ctx context.Context, username string, password []byte) ([]byte, error)
	// GenerateServiceToken generates a new service token
	GenerateServiceToken(ctx context.Context, description string, ttl time.Duration) (*api.ServiceToken, error)
	// UnaryServerInterceptor is the default interceptor
	UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
	// StreamServerInterceptor is the streaming server interceptor
	StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
}
