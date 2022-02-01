package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"git.underland.io/ehazlett/fynca/version"
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
	app.Usage = "fynca rendering system cli"
	app.Version = version.Version
	app.Authors = []*cli.Author{
		{
			Name: "@ehazlett",
		},
	}
	app.Commands = []*cli.Command{
		loginCommand,
		accountsCommand,
		queueJobCommand,
		workersCommand,
	}
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
		},
		&cli.StringFlag{
			Name:    "addr",
			Aliases: []string{"a"},
			Usage:   "fynca server address",
			Value:   "127.0.0.1:8080",
			EnvVars: []string{"FINCA_ADDR"},
		},
		&cli.StringFlag{
			Name:    "cert",
			Aliases: []string{"c"},
			Usage:   "fynca client certificate",
			Value:   "",
		},
		&cli.StringFlag{
			Name:  "key, k",
			Usage: "fynca client key",
			Value: "",
		},
		&cli.BoolFlag{
			Name:  "skip-verify",
			Usage: "skip TLS verification",
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
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
