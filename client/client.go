package client

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/ehazlett/flow"
	accountsapi "github.com/ehazlett/flow/api/services/accounts/v1"
	infoapi "github.com/ehazlett/flow/api/services/info/v1"
	workflowsapi "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

type Client struct {
	accountsapi.AccountsClient
	infoapi.InfoClient
	workflowsapi.WorkflowsClient
	config *flow.Config
	conn   *grpc.ClientConn
}

type ClientConfig struct {
	Username     string
	Namespace    string
	Token        string
	APIToken     string
	ServiceToken string
	Insecure     bool
}

func NewClient(cfg *flow.Config, clientOpts ...ClientOpt) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	clientConfig := &ClientConfig{}
	for _, opt := range clientOpts {
		opt(clientConfig)
	}

	opts, err := DialOptionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	if clientConfig.Insecure {
		opts = append(opts, grpc.WithInsecure())
	}

	opts = append(opts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(flow.GRPCMaxMessageSize),
			grpc.MaxCallSendMsgSize(flow.GRPCMaxMessageSize),
		),
	)

	// interceptors
	unaryClientInterceptors := []grpc.UnaryClientInterceptor{}
	streamClientInterceptors := []grpc.StreamClientInterceptor{}

	// auth
	authenticator := newClientAuthenticator(clientConfig)
	unaryClientInterceptors = append(unaryClientInterceptors, authenticator.authUnaryInterceptor)
	streamClientInterceptors = append(streamClientInterceptors, authenticator.authStreamInterceptor)

	// telemetry
	unaryClientInterceptors = append(unaryClientInterceptors, otelgrpc.UnaryClientInterceptor())
	streamClientInterceptors = append(streamClientInterceptors, otelgrpc.StreamClientInterceptor())

	opts = append(opts,
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unaryClientInterceptors...)),
		grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(streamClientInterceptors...)),
	)

	c, err := grpc.DialContext(ctx,
		cfg.GRPCAddress,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	client := &Client{
		accountsapi.NewAccountsClient(c),
		infoapi.NewInfoClient(c),
		workflowsapi.NewWorkflowsClient(c),
		cfg,
		c,
	}

	return client, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// DialOptionsFromConfig returns dial options configured from a Carbon config
func DialOptionsFromConfig(cfg *flow.Config) ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{}
	if cfg.EnableTLS {
		creds := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	// client cert
	if cfg.TLSClientCertificate != "" {
		logrus.WithField("cert", cfg.TLSClientCertificate)
		var creds credentials.TransportCredentials
		if cfg.TLSClientKey != "" {
			logrus.WithField("key", cfg.TLSClientKey)
			cert, err := tls.LoadX509KeyPair(cfg.TLSClientCertificate, cfg.TLSClientKey)
			if err != nil {
				return nil, err
			}
			creds = credentials.NewTLS(&tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
			})
		} else {
			c, err := credentials.NewClientTLSFromFile(cfg.TLSClientCertificate, "")
			if err != nil {
				return nil, err
			}
			creds = c
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	return opts, nil
}
