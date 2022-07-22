package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	accountsapi "github.com/ehazlett/flow/api/services/accounts/v1"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"golang.org/x/term"
)

var accountsCommand = &cli.Command{
	Name:  "accounts",
	Usage: "account management",
	Subcommands: []*cli.Command{
		accountsCreateCommand,
		accountsProfileCommand,
		accountsGenerateAPITokenCommand,
		accountsChangePasswordCommand,
		accountsGenerateServiceTokenCommand,
		accountsListServiceTokensCommand,
	},
}

var accountsCreateCommand = &cli.Command{
	Name:  "create",
	Usage: "create a new flow account",
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
		ctx, err := getContext(clix)
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

var accountsProfileCommand = &cli.Command{
	Name:  "profile",
	Usage: "view account profile",
	Flags: []cli.Flag{},
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

		resp, err := client.GetAccountProfile(ctx, &accountsapi.GetAccountProfileRequest{})
		if err != nil {
			return err
		}

		if err := json.NewEncoder(os.Stdout).Encode(resp.Account); err != nil {
			return err
		}

		return nil
	},
}

var accountsChangePasswordCommand = &cli.Command{
	Name:  "change-password",
	Usage: "change flow account password",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "account username",
		},
	},
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

var accountsGenerateServiceTokenCommand = &cli.Command{
	Name:  "generate-service-token",
	Usage: "create a new global service token",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "description",
			Usage: "token description",
			Value: "",
		},
		&cli.DurationFlag{
			Name:  "ttl",
			Usage: "ttl for service token",
			Value: 8760 * time.Hour, // 1 year
		},
	},
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

		resp, err := client.GenerateServiceToken(ctx, &accountsapi.GenerateServiceTokenRequest{
			Description: clix.String("description"),
			TTL:         clix.Duration("ttl"),
		})
		if err != nil {
			return err
		}

		fmt.Println(resp.ServiceToken.Token)

		return nil
	},
}

var accountsGenerateAPITokenCommand = &cli.Command{
	Name:  "generate-api-token",
	Usage: "create a new API token for your account",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "description",
			Usage: "token description",
			Value: "",
		},
	},
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

		resp, err := client.GenerateAPIToken(ctx, &accountsapi.GenerateAPITokenRequest{
			Description: clix.String("description"),
		})
		if err != nil {
			return err
		}

		fmt.Println(resp.APIToken.Token)

		return nil
	},
}

var accountsListServiceTokensCommand = &cli.Command{
	Name:  "list-service-tokens",
	Usage: "view service tokens",
	Flags: []cli.Flag{},
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

		resp, err := client.ListServiceTokens(ctx, &accountsapi.ListServiceTokensRequest{})
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
		fmt.Fprintf(w, "TOKEN\tCREATED\tLAST ACCESSED\n")
		for _, t := range resp.ServiceTokens {
			accessed := t.AccessedAt.Format(time.RFC3339)
			if t.AccessedAt.IsZero() {
				accessed = "never"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				t.Token,
				t.CreatedAt.Format(time.RFC3339),
				accessed,
			)
		}
		w.Flush()
		return nil
	},
}
