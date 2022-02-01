package auth

import (
	"context"

	api "git.underland.io/ehazlett/fynca/api/services/accounts/v1"
	"google.golang.org/grpc"
)

const (
	// AuthenticateMethod is the GRPC method that is used for client authentication
	AuthenticateMethod = "/fynca.services.accounts.v1.Accounts/Authenticate"
)

type Authenticator interface {
	// Name returns the name of the authenticator
	Name() string
	// GetAccount gets a Fynca account from the authenticator
	GetAccount(ctx context.Context, username string) (*api.Account, error)
	// Authenticate authenticates the specified user and returns a byte array from the provider
	Authenticate(ctx context.Context, username string, password []byte) ([]byte, error)
	// UnaryServerInterceptor is the default interceptor
	UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
	// StreamServerInterceptor is the streaming server interceptor
	StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
}
