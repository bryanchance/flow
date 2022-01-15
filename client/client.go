package client

import (
	"context"
	"crypto/tls"
	"time"

	"git.underland.io/ehazlett/finca"
	renderapi "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Client struct {
	renderapi.RenderClient
	config *finca.Config
	conn   *grpc.ClientConn
}

func NewClient(cfg *finca.Config) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	opts, err := DialOptionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	if len(opts) == 0 {
		opts = []grpc.DialOption{
			grpc.WithInsecure(),
		}
	}

	c, err := grpc.DialContext(ctx,
		cfg.GRPCAddress,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	client := &Client{
		renderapi.NewRenderClient(c),
		cfg,
		c,
	}

	return client, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// DialOptionsFromConfig returns dial options configured from a Carbon config
func DialOptionsFromConfig(cfg *finca.Config) ([]grpc.DialOption, error) {
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
