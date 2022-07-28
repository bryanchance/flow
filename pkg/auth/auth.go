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
	// Logout removes the authenticator key for the specified token
	Logout(ctx context.Context) error
	// GenerateAPIToken generates a new user API token
	GenerateAPIToken(ctx context.Context, description string) (*api.APIToken, error)
	// GenerateServiceToken generates a new service token
	GenerateServiceToken(ctx context.Context, description string, ttl time.Duration) (*api.ServiceToken, error)
	// ListServiceTokens returns all generated service tokens
	ListServiceTokens(ctx context.Context) ([]*api.ServiceToken, error)
	// UnaryServerInterceptor is the default interceptor
	UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
	// StreamServerInterceptor is the streaming server interceptor
	StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
	// ValidateAPIToken returns a valid account from the specified api token or an error if invalid
	ValidateAPIToken(ctx context.Context, token string) (*api.Account, error)
}
