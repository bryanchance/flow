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
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
	"github.com/ehazlett/flow/datastore"
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
	Token     string `json:"token,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// TokenAuthenticator is an authenticator that performs no authentication at all
type TokenAuthenticator struct {
	ds           datastore.Datastore
	publicRoutes map[string]struct{}
}

func NewTokenAuthenticator(ds datastore.Datastore, publicRoutes []string) *TokenAuthenticator {
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
			return nil, status.Errorf(codes.Unauthenticated, "invalid username or password")
		}
		return nil, errors.Wrap(err, "error comparing password hash")
	}

	token := flow.GenerateToken(account.Username)
	tokenKey := getTokenKey(token)
	if err := a.ds.SetAuthenticatorKey(ctx, a, tokenKey, []byte(account.Username), tokenTTL); err != nil {
		return nil, errors.Wrapf(err, "error saving token for %s", account.Username)
	}

	config := &Config{
		Token:     token,
		Namespace: account.CurrentNamespace,
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
		logrus.Warnf("missing metadata in context from %s", peer.Addr)
		return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}

	// check for auth token
	token, ok := metadata[flow.CtxTokenKey]
	if ok {
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
		namespace := account.CurrentNamespace
		// if namespace specified in context use it
		ns, ok := metadata[flow.CtxNamespaceKey]
		if ok {
			if v := ns[0]; v != "" {
				namespace = v
			}
		}
		// verify user is a member of specified namespace
		validNS, err := a.validateUsernameNamespace(ctx, account, namespace)
		if err != nil {
			return nil, errors.Wrapf(err, "error validating user namespace %s", namespace)
		}
		if !validNS {
			logrus.Warnf("unauthorized request from %s to namespace %s", peer.Addr, namespace)
			return nil, status.Errorf(codes.PermissionDenied, "access denied")
		}
		tCtx := context.WithValue(ctx, flow.CtxTokenKey, token)
		uCtx := context.WithValue(tCtx, flow.CtxUsernameKey, account.Username)
		aCtx := context.WithValue(uCtx, flow.CtxAdminKey, account.Admin)
		fCtx := context.WithValue(aCtx, flow.CtxNamespaceKey, namespace)
		return handler(fCtx, req)
	}

	// api token
	apiToken, ok := metadata[flow.CtxAPITokenKey]
	if ok {
		var account *api.Account
		for _, t := range apiToken {
			acct, err := a.ValidateAPIToken(ctx, t)
			if err != nil {
				logrus.Warnf("unauthenticated request from %s using api token %s", peer.Addr, t)
				return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
			}

			account = acct
			break
		}
		if account == nil {
			logrus.Warnf("unauthenticated request from %s", peer.Addr)
			return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
		}
		namespace := account.CurrentNamespace
		// if namespace specified in context use it
		ns, ok := metadata[flow.CtxNamespaceKey]
		if ok {
			if v := ns[0]; v != "" {
				namespace = v
			}
		}
		// verify user is a member of specified namespace
		validNS, err := a.validateUsernameNamespace(ctx, account, namespace)
		if err != nil {
			return nil, errors.Wrapf(err, "error validating user namespace %s", namespace)
		}
		if !validNS {
			logrus.Warnf("unauthorized request from %s to namespace %s", peer.Addr, namespace)
			return nil, status.Errorf(codes.PermissionDenied, "access denied")
		}
		tCtx := context.WithValue(ctx, flow.CtxTokenKey, token)
		uCtx := context.WithValue(tCtx, flow.CtxUsernameKey, account.Username)
		aCtx := context.WithValue(uCtx, flow.CtxAdminKey, account.Admin)
		fCtx := context.WithValue(aCtx, flow.CtxNamespaceKey, namespace)
		return handler(fCtx, req)
	}

	// service token
	nt, ok := metadata[flow.CtxServiceTokenKey]
	if ok {
		var serviceToken *api.ServiceToken
		for _, t := range nt {
			nToken, err := a.validateServiceToken(ctx, t)
			if err != nil {
				logrus.Warnf("unauthenticated request from %s using service token %s: %s", peer.Addr, t, err)
				return nil, status.Errorf(codes.Unauthenticated, "invalid or missing service token")
			}

			serviceToken = nToken
			break
		}
		if serviceToken == nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid or missing service token")
		}
		namespace := ""
		// if namespace specified in context use it
		ns, ok := metadata[flow.CtxNamespaceKey]
		if ok {
			logrus.Debugf("adding namespace for service token: %q", ns)
			if v := ns[0]; v != "" {
				namespace = v
			}
		}

		tCtx := context.WithValue(ctx, flow.CtxServiceTokenKey, serviceToken.Token)
		fCtx := context.WithValue(tCtx, flow.CtxNamespaceKey, namespace)
		return handler(fCtx, req)
	}
	logrus.Warnf("unauthenticated request from %s (missing token and service token)", peer.Addr)
	return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
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
		logrus.Warnf("missing metadata in context from %s", peer.Addr)
		return status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}

	// check for auth token
	token, ok := metadata[flow.CtxTokenKey]
	if ok {
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
		namespace := account.CurrentNamespace
		// if namespace specified in context use it
		ns, ok := metadata[flow.CtxNamespaceKey]
		if ok {
			if v := ns[0]; v != "" {
				namespace = v
			}
		}
		// verify user is a member of specified namespace
		validNS, err := a.validateUsernameNamespace(ctx, account, namespace)
		if err != nil {
			return errors.Wrapf(err, "error validating user namespace %s", namespace)
		}
		if !validNS {
			logrus.Warnf("unauthorized request from %s to namespace %s", peer.Addr, namespace)
			return status.Errorf(codes.PermissionDenied, "access denied")
		}
		tCtx := context.WithValue(ctx, flow.CtxTokenKey, token)
		uCtx := context.WithValue(tCtx, flow.CtxUsernameKey, account.Username)
		aCtx := context.WithValue(uCtx, flow.CtxAdminKey, account.Admin)
		fCtx := context.WithValue(aCtx, flow.CtxNamespaceKey, namespace)
		s.WrappedContext = fCtx
		return handler(srv, s)
	}

	// api token
	apiToken, ok := metadata[flow.CtxAPITokenKey]
	if ok {
		var account *api.Account
		for _, t := range apiToken {
			acct, err := a.ValidateAPIToken(ctx, t)
			if err != nil {
				logrus.Warnf("unauthenticated request from %s using api token %s", peer.Addr, t)
				return status.Errorf(codes.Unauthenticated, "invalid or missing token")
			}

			account = acct
			break
		}
		if account == nil {
			logrus.Warnf("unauthenticated request from %s", peer.Addr)
			return status.Errorf(codes.Unauthenticated, "invalid or missing token")
		}
		namespace := account.CurrentNamespace
		// if namespace specified in context use it
		ns, ok := metadata[flow.CtxNamespaceKey]
		if ok {
			if v := ns[0]; v != "" {
				namespace = v
			}
		}
		// verify user is a member of specified namespace
		validNS, err := a.validateUsernameNamespace(ctx, account, namespace)
		if err != nil {
			return errors.Wrapf(err, "error validating user namespace %s", namespace)
		}
		if !validNS {
			logrus.Warnf("unauthorized request from %s to namespace %s", peer.Addr, namespace)
			return status.Errorf(codes.PermissionDenied, "access denied")
		}
		tCtx := context.WithValue(ctx, flow.CtxAPITokenKey, apiToken)
		uCtx := context.WithValue(tCtx, flow.CtxUsernameKey, account.Username)
		aCtx := context.WithValue(uCtx, flow.CtxAdminKey, account.Admin)
		fCtx := context.WithValue(aCtx, flow.CtxNamespaceKey, namespace)
		s.WrappedContext = fCtx
		return handler(srv, s)
	}

	// service token
	nt, ok := metadata[flow.CtxServiceTokenKey]
	if ok {
		var serviceToken *api.ServiceToken
		for _, t := range nt {
			nToken, err := a.validateServiceToken(ctx, t)
			if err != nil {
				logrus.Warnf("unauthenticated request from %s using service token %s: %s", peer.Addr, t, err)
				return status.Errorf(codes.Unauthenticated, "invalid or missing service token")
			}

			serviceToken = nToken
			break
		}
		if serviceToken == nil {
			return status.Errorf(codes.Unauthenticated, "invalid or missing service token")
		}
		namespace := ""
		// if namespace specified in context use it
		ns, ok := metadata[flow.CtxNamespaceKey]
		if ok {
			logrus.Debugf("adding namespace for service token: %q", ns)
			if v := ns[0]; v != "" {
				namespace = v
			}
		}
		tCtx := context.WithValue(ctx, flow.CtxServiceTokenKey, serviceToken.Token)
		fCtx := context.WithValue(tCtx, flow.CtxNamespaceKey, namespace)
		s.WrappedContext = fCtx
		return handler(srv, s)
	}

	logrus.Warnf("unauthenticated request from %s (missing token and service token)", peer.Addr)
	return status.Errorf(codes.Unauthenticated, "invalid or missing token and service token")
}

func (a *TokenAuthenticator) GetAccount(ctx context.Context, token string) (*api.Account, error) {
	return a.validateToken(ctx, token)
}

func (a *TokenAuthenticator) GenerateServiceToken(ctx context.Context, description string, ttl time.Duration) (*api.ServiceToken, error) {
	// generate service token
	token := flow.GenerateToken(time.Now().String())
	serviceToken, err := a.createServiceToken(ctx, token, description, ttl)
	if err != nil {
		return nil, err
	}

	return serviceToken, nil
}

func (a *TokenAuthenticator) ListServiceTokens(ctx context.Context) ([]*api.ServiceToken, error) {
	serviceTokens, err := a.ds.GetServiceTokens(ctx)
	if err != nil {
		return nil, err
	}

	return serviceTokens, nil
}

func (a *TokenAuthenticator) GenerateAPIToken(ctx context.Context, description string) (*api.APIToken, error) {
	token := flow.GenerateToken(time.Now().String())
	apiToken, err := a.createAPIToken(ctx, token, description)
	if err != nil {
		return nil, err
	}

	return apiToken, nil
}

func (a *TokenAuthenticator) createServiceToken(ctx context.Context, token string, description string, ttl time.Duration) (*api.ServiceToken, error) {
	serviceToken := &api.ServiceToken{
		Token:       token,
		Description: description,
		CreatedAt:   time.Now(),
	}

	if err := a.ds.CreateServiceToken(ctx, serviceToken); err != nil {
		return nil, errors.Wrap(err, "error saving service token")
	}

	return serviceToken, nil
}

func (a *TokenAuthenticator) createAPIToken(ctx context.Context, token string, description string) (*api.APIToken, error) {
	// lookup requesting user to assign token
	userToken, err := flow.GetTokenFromContext(ctx)
	if err != nil {
		return nil, err
	}
	acct, err := a.validateToken(ctx, userToken)
	if err != nil {
		return nil, err
	}

	apiToken := &api.APIToken{
		ID:          acct.ID,
		Token:       token,
		Description: description,
		CreatedAt:   time.Now(),
	}

	if err := a.ds.CreateAPIToken(ctx, apiToken); err != nil {
		return nil, errors.Wrap(err, "error saving api token")
	}

	return apiToken, nil
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

func (a *TokenAuthenticator) ValidateAPIToken(ctx context.Context, token string) (*api.Account, error) {
	apiToken, err := a.ds.GetAPIToken(ctx, token)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting api token %s from datastore", token)
	}

	acct, err := a.ds.GetAccountByID(ctx, apiToken.ID)
	if err != nil {
		return nil, err
	}

	return acct, err
}

func (a *TokenAuthenticator) validateServiceToken(ctx context.Context, token string) (*api.ServiceToken, error) {
	serviceToken, err := a.ds.GetServiceToken(ctx, token)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting service token %s from datastore", token)
	}

	// update accessedAt
	serviceToken.AccessedAt = time.Now()
	if err := a.ds.UpdateServiceToken(ctx, serviceToken); err != nil {
		return nil, err
	}

	return serviceToken, nil
}

func (a *TokenAuthenticator) validateUsernameNamespace(ctx context.Context, acct *api.Account, nsID string) (bool, error) {
	ns, err := a.ds.GetNamespace(ctx, nsID)
	if err != nil {
		return false, err
	}
	if ns.OwnerID == acct.ID {
		return true, nil
	}

	for _, id := range ns.Members {
		if id == acct.ID {
			return true, nil
		}
	}
	return false, nil
}

func getTokenKey(token string) string {
	return path.Join("tokens", token)
}

func getServiceTokenKey(token string) string {
	return path.Join("servicetokens", token)
}

func getAPITokenKey(token string) string {
	return path.Join("apitokens", token)
}
