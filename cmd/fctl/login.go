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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	accountsapi "github.com/fynca/fynca/api/services/accounts/v1"
	renderapi "github.com/fynca/fynca/api/services/jobs/v1"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/term"
)

type tokenConfig struct {
	Token string `json:"token"`
}

var loginCommand = &cli.Command{
	Name:  "login",
	Usage: "login to fynca server",
	Action: func(clix *cli.Context) error {
		ctx := context.Background()
		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		v, err := client.Version(ctx, &renderapi.VersionRequest{})
		if err != nil {
			return err
		}

		logrus.Debugf("%+v", v)

		rdr := bufio.NewReader(os.Stdin)
		fmt.Print("Username: ")
		username, err := rdr.ReadString('\n')
		if err != nil {
			return err
		}

		fmt.Print("Password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}
		fmt.Println()

		resp, err := client.Authenticate(ctx, &accountsapi.AuthenticateRequest{
			Username: strings.TrimSpace(username),
			Password: password,
		})
		if err != nil {
			return err
		}

		localPath, err := localConfigPath()
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
			return err
		}

		if err := os.WriteFile(localPath, resp.Config, 0600); err != nil {
			return err
		}

		fmt.Println("login successful")

		return nil
	},
}

func localConfigPath() (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".fynca", "config.json"), nil
}

func getLocalConfig() (*tokenConfig, error) {
	localPath, err := localConfigPath()
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
