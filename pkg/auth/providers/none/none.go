package none

import (
	"context"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/accounts/v1"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

var (
	authenticatorName = "none"
)

// NoneAuthenticator is an authenticator that performs no authentication at all
type NoneAuthenticator struct{}

func (a *NoneAuthenticator) Name() string {
	return authenticatorName
}

func (a *NoneAuthenticator) Authenticate(ctx context.Context, username string, password []byte) ([]byte, error) {
	return nil, nil
}

func (a *NoneAuthenticator) GetAccount(ctx context.Context, token string) (*api.Account, error) {
	return nil, nil
}

func (a *NoneAuthenticator) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// set default namespace
	aCtx := context.WithValue(ctx, fynca.CtxNamespaceKey, fynca.CtxDefaultNamespace)
	return handler(aCtx, req)
}

func (a *NoneAuthenticator) StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	// set default namespace
	aCtx := context.WithValue(ctx, fynca.CtxNamespaceKey, fynca.CtxDefaultNamespace)
	s := grpc_middleware.WrapServerStream(stream)
	s.WrappedContext = aCtx
	return handler(srv, s)
}
