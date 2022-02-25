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
	"log"
	"os"

	"github.com/fynca/fynca/version"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "fynca-worker"
	app.Version = version.FullVersion()
	app.Authors = []*cli.Author{
		{
			Name: "@ehazlett",
		},
	}
	app.Usage = "fynca job worker"
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			EnvVars: []string{"DEBUG"},
		},
		&cli.StringFlag{
			Name:  "id",
			Usage: "worker ID",
			Value: getHostname(),
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to fynca config",
			Value:   "fynca.toml",
		},
		&cli.StringFlag{
			Name:    "profiler-address",
			Aliases: []string{"p"},
			Usage:   "enable profiler on address",
			Value:   "",
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Action = runAction

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func getHostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}
