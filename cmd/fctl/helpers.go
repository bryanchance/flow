package main

import (
	"git.underland.io/ehazlett/finca"
	"git.underland.io/ehazlett/finca/client"
	cli "github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

func getClient(clix *cli.Context) (*client.Client, error) {
	opts := []grpc.DialOption{}
	cert := clix.String("cert")
	key := clix.String("key")
	skipVerification := clix.Bool("skip-verify")

	cfg := &finca.Config{
		TLSClientCertificate:  cert,
		TLSClientKey:          key,
		TLSInsecureSkipVerify: skipVerification,
	}

	opts, err := client.DialOptionsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return client.NewClient(clix.String("addr"), opts...)
}
