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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"syscall"
	"time"

	accountsapi "github.com/fynca/fynca/api/services/accounts/v1"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/term"
)

var accountsCommand = &cli.Command{
	Name:  "accounts",
	Usage: "account management",
	Subcommands: []*cli.Command{
		accountsCreateCommand,
		accountsChangePasswordCommand,
	},
}

var accountsCreateCommand = &cli.Command{
	Name:  "create",
	Usage: "create a new fynca account",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "account username",
		},
		&cli.StringFlag{
			Name:    "email",
			Aliases: []string{"e"},
			Usage:   "account email",
		},
		&cli.StringFlag{
			Name:    "first-name",
			Aliases: []string{"f"},
			Usage:   "account first name",
		},
		&cli.StringFlag{
			Name:    "last-name",
			Aliases: []string{"l"},
			Usage:   "account last name",
		},
		&cli.BoolFlag{
			Name:  "admin",
			Usage: "admin privileges",
		},
	},
	Action: func(clix *cli.Context) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		// initial password
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s", time.Now())))
		tmpPassword := hex.EncodeToString(hash[:10])

		acct := &accountsapi.Account{
			Username:  clix.String("username"),
			FirstName: clix.String("first-name"),
			LastName:  clix.String("last-name"),
			Email:     clix.String("email"),
			Admin:     clix.Bool("admin"),
			Password:  tmpPassword,
		}

		if _, err := client.CreateAccount(ctx, &accountsapi.CreateAccountRequest{
			Account: acct,
		}); err != nil {
			return err
		}

		logrus.Infof("initial password: %s", tmpPassword)
		logrus.Infof("created account %s successfully", acct.Username)

		return nil
	},
}

var accountsChangePasswordCommand = &cli.Command{
	Name:  "change-password",
	Usage: "change fynca account password",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "account username",
		},
	},
	Action: func(clix *cli.Context) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		username := clix.String("username")

		if username == "" {
			cli.ShowSubcommandHelp(clix)
			return fmt.Errorf("username must be specified")
		}

		fmt.Print("Password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}
		fmt.Println()

		fmt.Print("Password (again): ")
		passwordAgain, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}
		fmt.Println()

		if string(passwordAgain) != string(password) {
			return fmt.Errorf("passwords do not match")
		}

		if _, err := client.ChangePassword(ctx, &accountsapi.ChangePasswordRequest{
			Username: username,
			Password: password,
		}); err != nil {
			return err
		}

		logrus.Infof("password changed for %s successfully", username)

		return nil
	},
}
