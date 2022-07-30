package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/ehazlett/flow/version"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SimpleFormatter struct{}

func (s *SimpleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}
	switch entry.Level {
	case logrus.ErrorLevel, logrus.FatalLevel:
		fmt.Fprintf(b, "ERR: ")
	}
	fmt.Fprintf(b, entry.Message)
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func main() {
	app := cli.NewApp()
	app.Name = "fctl"
	app.Usage = "flow workflow system cli"
	app.Version = version.Version
	app.Authors = []*cli.Author{
		{
			Name: "@ehazlett",
		},
	}
	app.Commands = []*cli.Command{
		loginCommand,
		logoutCommand,
		accountsCommand,
		infoCommand,
		workflowsCommand,
	}
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
		},
		&cli.StringFlag{
			Name:    "addr",
			Aliases: []string{"a"},
			Usage:   "flow server address",
			Value:   "127.0.0.1:7080",
			EnvVars: []string{"FLOW_ADDR"},
		},
		&cli.BoolFlag{
			Name:    "tls",
			Usage:   "enable TLS",
			EnvVars: []string{"FLOW_ENABLE_TLS"},
		},
		&cli.StringFlag{
			Name:    "cert",
			Aliases: []string{"c"},
			Usage:   "flow client certificate",
			EnvVars: []string{"FLOW_CLIENT_CERT"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "key",
			Aliases: []string{"k"},
			Usage:   "flow client key",
			EnvVars: []string{"FLOW_CLIENT_KEY"},
			Value:   "",
		},
		&cli.BoolFlag{
			Name:    "skip-verify",
			Usage:   "skip TLS verification",
			EnvVars: []string{"FLOW_INSECURE_SKIP_VERIFY"},
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if v := clix.String("addr"); v == "" {
			return fmt.Errorf("addr cannot be empty")
		}
		logrus.SetFormatter(&SimpleFormatter{})
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.Unauthenticated:
				logrus.Fatal(errors.New("Invalid or expired authentication.  Please login."))
			case codes.PermissionDenied:
				logrus.Fatal(errors.New("Access denied"))
			default:
				logrus.Fatal(e.Message())
			}
		}
		logrus.Fatal(err)
	}
}
