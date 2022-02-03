package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/accounts/v1"
	"git.underland.io/ehazlett/fynca/pkg/auth"
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
	// check for token
	token, ok := metadata[fynca.CtxTokenKey]
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "invalid or missing token")
	}
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
