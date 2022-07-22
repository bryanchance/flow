package none

import (
	"context"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
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

func (a *NoneAuthenticator) GenerateAPIToken(ctx context.Context, description string) (*api.APIToken, error) {
	return nil, nil
}

func (a *NoneAuthenticator) GenerateServiceToken(ctx context.Context, description string, ttl time.Duration) (*api.ServiceToken, error) {
	return nil, nil
}

func (a *NoneAuthenticator) ListServiceTokens(ctx context.Context) ([]*api.ServiceToken, error) {
	return nil, nil
}

func (a *NoneAuthenticator) ValidateAPIToken(ctx context.Context, token string) (*api.Account, error) {
	return nil, nil
}

func (a *NoneAuthenticator) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// set default namespace
	aCtx := context.WithValue(ctx, flow.CtxNamespaceKey, flow.CtxDefaultNamespace)
	return handler(aCtx, req)
}

func (a *NoneAuthenticator) StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	// set default namespace
	aCtx := context.WithValue(ctx, flow.CtxNamespaceKey, flow.CtxDefaultNamespace)
	s := grpc_middleware.WrapServerStream(stream)
	s.WrappedContext = aCtx
	return handler(srv, s)
}
