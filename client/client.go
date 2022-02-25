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
package client

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/fynca/fynca"
	accountsapi "github.com/fynca/fynca/api/services/accounts/v1"
	jobsapi "github.com/fynca/fynca/api/services/jobs/v1"
	workersapi "github.com/fynca/fynca/api/services/workers/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

type Client struct {
	jobsapi.JobsClient
	accountsapi.AccountsClient
	workersapi.WorkersClient
	config *fynca.Config
	conn   *grpc.ClientConn
}

type ClientConfig struct {
	Username string
	Token    string
}

func NewClient(cfg *fynca.Config, clientOpts ...ClientOpt) (*Client, error) {
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

	if len(opts) == 0 {
		opts = []grpc.DialOption{
			grpc.WithInsecure(),
		}
	}

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
		//grpc.WithUnaryInterceptor(authenticator.authUnaryInterceptor),
		//grpc.WithStreamInterceptor(authenticator.authStreamInterceptor),
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
		jobsapi.NewJobsClient(c),
		accountsapi.NewAccountsClient(c),
		workersapi.NewWorkersClient(c),
		cfg,
		c,
	}

	return client, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// DialOptionsFromConfig returns dial options configured from a Carbon config
func DialOptionsFromConfig(cfg *fynca.Config) ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{}
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
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	return opts, nil
}
