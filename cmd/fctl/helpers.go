package main

import (
	"git.underland.io/ehazlett/finca"
	"git.underland.io/ehazlett/finca/client"
	cli "github.com/urfave/cli/v2"
)

func getClient(clix *cli.Context) (*client.Client, error) {
	cert := clix.String("cert")
	key := clix.String("key")
	skipVerification := clix.Bool("skip-verify")

	cfg := &finca.Config{
		GRPCAddress:           clix.String("addr"),
		TLSClientCertificate:  cert,
		TLSClientKey:          key,
		TLSInsecureSkipVerify: skipVerification,
	}

	return client.NewClient(cfg)
}
