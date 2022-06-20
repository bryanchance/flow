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

	"github.com/ehazlett/flow/version"
	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "fynca"
	app.Version = version.FullVersion()
	app.Authors = []*cli.Author{
		{
			Name: "@ehazlett",
		},
	}
	app.Usage = "distributed render manager"
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			Usage:   "enable debug logging",
		},
		&cli.StringFlag{
			Name:    "profiler-address",
			Aliases: []string{"p"},
			Usage:   "enable profiler on this address",
			Value:   "",
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Commands = []*cli.Command{
		configCommand,
		serverCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

var configCommand = &cli.Command{
	Name:  "config",
	Usage: "generate fynca configuration",
	Flags: []cli.Flag{},
	Action: func(clix *cli.Context) error {
		cfg, err := defaultConfig(clix)
		if err != nil {
			return err
		}
		if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
			return err
		}
		return nil
	},
}
