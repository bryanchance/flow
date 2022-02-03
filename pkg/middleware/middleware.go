package middleware

import (
	"context"

	"google.golang.org/grpc"
)

type Middleware interface {
	// Name returns the name of the authenticator
	Name() string
	// UnaryServerInterceptor is the default interceptor
	UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
	// StreamServerInterceptor is the streaming server interceptor
	StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
}
