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
package main

import (
	"context"

	"github.com/fynca/fynca"
	"github.com/fynca/fynca/client"
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
