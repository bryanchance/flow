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
package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
	"github.com/ehazlett/flow/pkg/auth"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var (
	name = "adminRequired"
)

// AdminRequired is an authenticator that performs no authentication at all
type AdminRequired struct {
	authenticator auth.Authenticator
	adminRoutes   map[string]struct{}
	publicRoutes  map[string]struct{}
}

func NewAdminRequired(a auth.Authenticator, adminRoutes, publicRoutes []string) *AdminRequired {
	ar := map[string]struct{}{}
	for _, route := range adminRoutes {
		ar[route] = struct{}{}
	}
	pr := map[string]struct{}{}
	for _, route := range publicRoutes {
		pr[route] = struct{}{}
	}
	return &AdminRequired{
		authenticator: a,
		adminRoutes:   ar,
		publicRoutes:  pr,
	}
}

func (m *AdminRequired) Name() string {
	return name
}

func (m *AdminRequired) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		logrus.Warn("failed to get peer from context")
	}

	if m.isPublicRoute(info.FullMethod) {
		return handler(ctx, req)
	}

	account, err := m.getAccount(ctx)
	if err != nil {
		logrus.WithError(err).Warnf("unauthorized request from %s", peer.Addr)
		return nil, err
	}

	// check route
	if m.adminRouteMatch(info.FullMethod) {
		if !account.Admin {
			logrus.Warnf("unauthorized request to %s from %s", info.FullMethod, peer.Addr)
			return nil, status.Errorf(codes.PermissionDenied, "access denied")
		}
	}
	return handler(ctx, req)
}

func (m *AdminRequired) StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	s := grpc_middleware.WrapServerStream(stream)

	peer, ok := peer.FromContext(ctx)
	if !ok {
		logrus.Warn("failed to get peer from context")
	}

	if m.isPublicRoute(info.FullMethod) {
		return handler(srv, s)
	}

	account, err := m.getAccount(ctx)
	if err != nil {
		logrus.WithError(err).Warnf("unauthorized request from %s", peer.Addr)
		return err
	}

	// check route
	if m.adminRouteMatch(info.FullMethod) {
		if !account.Admin {
			logrus.Warnf("unauthorized request to %s from %s", info.FullMethod, peer.Addr)
			return status.Errorf(codes.PermissionDenied, "access denied")
		}
	}
	s.WrappedContext = ctx
	return handler(srv, s)
}

func (m *AdminRequired) getAccount(ctx context.Context) (*api.Account, error) {
	metadata, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "invalid or missing token")
	}
	// check for service token
	if _, ok := metadata[flow.CtxServiceTokenKey]; ok {
		return nil, nil
	}

	// check for token
	token, ok := metadata[flow.CtxTokenKey]
	if ok {
		var account *api.Account
		for _, t := range token {
			acct, err := m.authenticator.GetAccount(ctx, t)
			if err != nil {
				return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
			}

			account = acct
			break
		}
		if account == nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
		}
		return account, nil
	}

	apiToken, ok := metadata[flow.CtxAPITokenKey]
	if ok {
		for _, t := range apiToken {
			acct, err := m.authenticator.ValidateAPIToken(ctx, t)
			if err != nil {
				return nil, status.Errorf(codes.Unauthenticated, "invalid api token")
			}
			return acct, nil
		}
	}

	return nil, status.Errorf(codes.Unauthenticated, "invalid or missing api token")
}

func (m *AdminRequired) adminRouteMatch(method string) bool {
	p := strings.Split(method, ".")
	if len(p) == 0 {
		return false
	}
	path := p[len(p)-1]
	if _, ok := m.adminRoutes[path]; ok {
		return true
	}
	return false
}

func (m *AdminRequired) isPublicRoute(method string) bool {
	p := strings.Split(method, ".")
	if len(p) == 0 {
		return false
	}
	path := p[len(p)-1]
	if _, ok := m.publicRoutes[path]; ok {
		return true
	}
	return false
}

func getTokenKey(token string) string {
	return path.Join("tokens", token)
}

func generateToken(username string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", username, time.Now())))
	return hex.EncodeToString(hash[:])
}
