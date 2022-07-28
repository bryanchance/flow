package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	accountsapi "github.com/ehazlett/flow/api/services/accounts/v1"
	infoapi "github.com/ehazlett/flow/api/services/info/v1"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/term"
)

type tokenConfig struct {
	Token string `json:"token"`
}

var loginCommand = &cli.Command{
	Name:  "login",
	Usage: "login to flow server",
	Action: func(clix *cli.Context) error {
		ctx := context.Background()
		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		endpoint := clix.String("addr")

		v, err := client.Version(ctx, &infoapi.VersionRequest{})
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

		localPath, err := localConfigPath(endpoint)
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

var logoutCommand = &cli.Command{
	Name:  "logout",
	Usage: "logout from flow server",
	Action: func(clix *cli.Context) error {
		ctx, err := getContext(clix)
		if err != nil {
			return err
		}

		client, err := getClient(clix)
		if err != nil {
			return err
		}
		defer client.Close()

		endpoint := clix.String("addr")

		v, err := client.Version(ctx, &infoapi.VersionRequest{})
		if err != nil {
			return err
		}

		logrus.Debugf("%+v", v)

		if _, err := client.Logout(ctx, &accountsapi.LogoutRequest{}); err != nil {
			return err
		}

		localPath, err := localConfigPath(endpoint)
		if err != nil {
			return err
		}

		if err := os.Remove(localPath); err != nil {
			return err
		}

		fmt.Printf("logged out of %s successfully\n", endpoint)

		return nil
	},
}
