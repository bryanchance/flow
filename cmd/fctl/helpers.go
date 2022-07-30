package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ehazlett/flow"
	"github.com/ehazlett/flow/client"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/mitchellh/go-homedir"
	cli "github.com/urfave/cli/v2"
	"google.golang.org/grpc/metadata"
)

func localConfigPath(endpoint string) (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	epHash := flow.GenerateHash(endpoint)
	return filepath.Join(homeDir, ".flow", fmt.Sprintf("%s.json", epHash)), nil
}

func getLocalConfig(endpoint string) (*tokenConfig, error) {
	localPath, err := localConfigPath(endpoint)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}

	var cfg *tokenConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func getClient(clix *cli.Context) (*client.Client, error) {
	cert := clix.String("cert")
	key := clix.String("key")
	skipVerification := clix.Bool("skip-verify")

	cfg := &flow.Config{
		GRPCAddress:           clix.String("addr"),
		EnableTLS:             clix.Bool("tls"),
		TLSClientCertificate:  cert,
		TLSClientKey:          key,
		TLSInsecureSkipVerify: skipVerification,
	}

	return client.NewClient(cfg)
}

func getContext(clix *cli.Context) (context.Context, error) {
	endpoint := clix.String("addr")
	config, err := getLocalConfig(endpoint)
	if err != nil {
		return nil, err
	}
	md := metadata.New(map[string]string{"token": config.Token})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	return ctx, nil
}

func marshaler() *jsonpb.Marshaler {
	return &jsonpb.Marshaler{EmitDefaults: true}
}
