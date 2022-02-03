package main

import (
	"context"

	"git.underland.io/ehazlett/fynca"
	"git.underland.io/ehazlett/fynca/client"
	cli "github.com/urfave/cli/v2"
	"google.golang.org/grpc/metadata"
)

func getClient(clix *cli.Context) (*client.Client, error) {
	cert := clix.String("cert")
	key := clix.String("key")
	skipVerification := clix.Bool("skip-verify")

	cfg := &fynca.Config{
		GRPCAddress:           clix.String("addr"),
		TLSClientCertificate:  cert,
		TLSClientKey:          key,
		TLSInsecureSkipVerify: skipVerification,
	}

	return client.NewClient(cfg)
}

func getContext() (context.Context, error) {
	config, err := getLocalConfig()
	if err != nil {
		return nil, err
	}
	md := metadata.New(map[string]string{"token": config.Token})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	return ctx, nil
}
