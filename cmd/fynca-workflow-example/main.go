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
	"time"

	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	workflowName = "fynca-workflow-example"
	workflowType = "io.fynca.workflows.example"
)

func main() {
	app := cli.NewApp()
	app.Name = workflowName
	app.Usage = "fynca example workflow"
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
		&cli.DurationFlag{
			Name:    "task-timeout",
			Aliases: []string{"t"},
			Value:   8 * time.Hour,
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "fynca config path",
		},
		&cli.IntFlag{
			Name:    "max-tasks",
			Aliases: []string{"m"},
			Usage:   "maximum number of tasks to process",
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
