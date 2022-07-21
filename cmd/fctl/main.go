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
		&cli.StringFlag{
			Name:    "cert",
			Aliases: []string{"c"},
			Usage:   "flow client certificate",
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "key",
			Aliases: []string{"k"},
			Usage:   "flow client key",
			Value:   "",
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
