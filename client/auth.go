package client

import (
	"context"

	"github.com/ehazlett/flow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type clientAuthenticator struct {
	cfg *ClientConfig
}

func newClientAuthenticator(cfg *ClientConfig) *clientAuthenticator {
	return &clientAuthenticator{
		cfg: cfg,
	}
}

func (a *clientAuthenticator) authUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	kvs := []string{}
	if a.cfg.Token != "" {
		kvs = append(kvs, flow.CtxTokenKey, a.cfg.Token)
	}
	if a.cfg.APIToken != "" {
		kvs = append(kvs, flow.CtxAPITokenKey, a.cfg.APIToken)
	}
	if a.cfg.ServiceToken != "" {
		kvs = append(kvs, flow.CtxServiceTokenKey, a.cfg.ServiceToken)
	}
	if a.cfg.Namespace != "" {
		kvs = append(kvs, flow.CtxNamespaceKey, a.cfg.Namespace)
	}
	authCtx := metadata.AppendToOutgoingContext(ctx, kvs...)
	return invoker(authCtx, method, req, reply, cc, opts...)
}

func (a *clientAuthenticator) authStreamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	kvs := []string{}
	if a.cfg.Token != "" {
		kvs = append(kvs, flow.CtxTokenKey, a.cfg.Token)
	}
	if a.cfg.APIToken != "" {
		kvs = append(kvs, flow.CtxAPITokenKey, a.cfg.APIToken)
	}
	if a.cfg.ServiceToken != "" {
		kvs = append(kvs, flow.CtxServiceTokenKey, a.cfg.ServiceToken)
	}
	if a.cfg.Namespace != "" {
		kvs = append(kvs, flow.CtxNamespaceKey, a.cfg.Namespace)
	}
	authCtx := metadata.AppendToOutgoingContext(ctx, kvs...)
	return streamer(authCtx, desc, cc, method, opts...)
}
