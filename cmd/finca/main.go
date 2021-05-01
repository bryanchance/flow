package main

import (
	"log"
	"os"

	"git.underland.io/ehazlett/finca/version"
	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "finca"
	app.Version = version.Version
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
	Usage: "generate finca configuration",
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
