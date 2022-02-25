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
package token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/accounts/v1"
	"github.com/fynca/fynca/datastore"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var (
	// ErrInvalidUsernamePassword is returned on failed authentication
	ErrInvalidUsernamePassword = errors.New("invalid username or password")
	authenticatorName          = "token"
	// token valid for 1 day
	tokenTTL = time.Second * 86400
)

type Config struct {
	Token string `json:"token,omitempty"`
}

// TokenAuthenticator is an authenticator that performs no authentication at all
type TokenAuthenticator struct {
	ds           *datastore.Datastore
	publicRoutes map[string]struct{}
}

func NewTokenAuthenticator(ds *datastore.Datastore, publicRoutes []string) *TokenAuthenticator {
	pr := map[string]struct{}{}
	for _, r := range publicRoutes {
		pr[r] = struct{}{}
	}
	return &TokenAuthenticator{
		ds:           ds,
		publicRoutes: pr,
	}
}

func (a *TokenAuthenticator) Name() string {
	return authenticatorName
}

func (a *TokenAuthenticator) Authenticate(ctx context.Context, username string, password []byte) ([]byte, error) {
	account, err := a.ds.GetAccount(ctx, username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword(account.PasswordCrypt, password); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			logrus.Warnf("invalid username or password for %s", username)
			//return nil, ErrInvalidUsernamePassword
			return nil, status.Errorf(codes.Unauthenticated, "invalid username or password")
		}
		return nil, errors.Wrap(err, "error comparing password hash")
	}

	token := generateToken(account.Username)
	tokenKey := getTokenKey(token)
	if err := a.ds.SetAuthenticatorKey(ctx, a, tokenKey, []byte(account.Username), tokenTTL); err != nil {
		return nil, errors.Wrapf(err, "error saving token for %s", account.Username)
	}

	config := &Config{
		Token: token,
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (a *TokenAuthenticator) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		logrus.Warn("failed to get peer from context")
	}

	// allow public routes
	if a.isPublicRoute(info.FullMethod) {
		return handler(ctx, req)
	}

	metadata, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logrus.Warnf("unauthenticated request from %s", peer.Addr)
		return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}
	// check for token
	token, ok := metadata[fynca.CtxTokenKey]
	if !ok {
		logrus.Warnf("unauthenticated request from %s", peer.Addr)
		return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}
	var account *api.Account
	for _, t := range token {
		acct, err := a.validateToken(ctx, t)
		if err != nil {
			logrus.Warnf("unauthenticated request from %s using token %s", peer.Addr, t)
			return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
		}

		account = acct
		break
	}
	if account == nil {
		logrus.Warnf("unauthenticated request from %s", peer.Addr)
		return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}
	// TODO: support organizations
	namespace := account.ID
	tCtx := context.WithValue(ctx, fynca.CtxTokenKey, token)
	uCtx := context.WithValue(tCtx, fynca.CtxUsernameKey, account.Username)
	aCtx := context.WithValue(uCtx, fynca.CtxAdminKey, account.Admin)
	fCtx := context.WithValue(aCtx, fynca.CtxNamespaceKey, namespace)
	return handler(fCtx, req)
}

func (a *TokenAuthenticator) StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	s := grpc_middleware.WrapServerStream(stream)

	peer, ok := peer.FromContext(ctx)
	if !ok {
		logrus.Warn("failed to get peer from context")
	}

	// allow public routes
	if a.isPublicRoute(info.FullMethod) {
		return handler(srv, s)
	}

	metadata, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logrus.Warnf("unauthenticated request from %s", peer.Addr)
		return status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}
	// check for token
	token, ok := metadata[fynca.CtxTokenKey]
	if !ok {
		logrus.Warnf("unauthenticated request from %s", peer.Addr)
		return status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}
	var account *api.Account
	for _, t := range token {
		acct, err := a.validateToken(ctx, t)
		if err != nil {
			logrus.Warnf("unauthenticated request from %s using token %s", peer.Addr, t)
			return status.Errorf(codes.Unauthenticated, "invalid or missing token")
		}

		account = acct
		break
	}
	if account == nil {
		logrus.Warnf("unauthenticated request from %s", peer.Addr)
		return status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}
	// TODO: support organizations
	namespace := account.ID
	tCtx := context.WithValue(ctx, fynca.CtxTokenKey, token)
	uCtx := context.WithValue(tCtx, fynca.CtxUsernameKey, account.Username)
	aCtx := context.WithValue(uCtx, fynca.CtxAdminKey, account.Admin)
	fCtx := context.WithValue(aCtx, fynca.CtxNamespaceKey, namespace)
	s.WrappedContext = fCtx
	return handler(srv, s)
}

func (a *TokenAuthenticator) GetAccount(ctx context.Context, token string) (*api.Account, error) {
	return a.validateToken(ctx, token)

}

func (a *TokenAuthenticator) isPublicRoute(method string) bool {
	p := strings.Split(method, ".")
	if len(p) == 0 {
		return false
	}
	path := p[len(p)-1]
	if _, ok := a.publicRoutes[path]; ok {
		return true
	}
	return false
}

func (a *TokenAuthenticator) validateToken(ctx context.Context, token string) (*api.Account, error) {
	tokenKey := getTokenKey(token)
	data, err := a.ds.GetAuthenticatorKey(ctx, a, tokenKey)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting token %s from datastore", token)
	}

	// token valid; lookup user
	return a.ds.GetAccount(ctx, string(data))
}

func getTokenKey(token string) string {
	return path.Join("tokens", token)
}

func generateToken(username string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", username, time.Now())))
	return hex.EncodeToString(hash[:])
}
