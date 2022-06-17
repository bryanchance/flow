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
	"os"

	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	workflowName = "fynca-workflow-blender-render"
	workflowType = "io.fynca.workflows.blender/render"
)

func main() {
	app := cli.NewApp()
	app.Name = workflowName
	app.Usage = "fynca blender render plugin"
	app.Version = "0.1.0"
	app.Authors = []*cli.Author{
		{
			Name: "@ehazlett",
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Action = workerAction
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			Usage:   "enable debug logging",
		},
		&cli.StringFlag{
			Name:  "id",
			Usage: "worker id",
			Value: getHostname(),
		},
		&cli.StringFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Usage:   "fynca server grpc address",
			Value:   "127.0.0.1:7080",
		},
		&cli.StringFlag{
			Name:  "tls-certificate",
			Usage: "tls client certificate",
		},
		&cli.StringFlag{
			Name:  "tls-key",
			Usage: "tls client key",
		},
		&cli.BoolFlag{
			Name:  "tls-skip-verify",
			Usage: "tls skip verify",
		},
		&cli.StringFlag{
			Name:    "service-token",
			Aliases: []string{"t"},
			Usage:   "fynca service token for access",
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "DEPRECATED: fynca config path",
		},
		&cli.StringFlag{
			Name:    "blender-path",
			Aliases: []string{"b"},
			Usage:   "path to blender executable (default: find in $PATH)",
		},
		&cli.IntFlag{
			Name:    "max-workflows",
			Aliases: []string{"m"},
			Usage:   "maximum number of workflows to process",
			Value:   0,
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
