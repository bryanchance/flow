package main

import (
	"log"
	"os"

	"git.underland.io/ehazlett/finca/version"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "fctl"
	app.Usage = "finca rendering system cli"
	app.Version = version.Version
	app.Authors = []*cli.Author{
		{
			Name: "@ehazlett",
		},
	}
	app.Commands = []*cli.Command{
		queueJobCommand,
	}
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
		},
		&cli.StringFlag{
			Name:    "addr",
			Aliases: []string{"a"},
			Usage:   "finca server address",
			Value:   "127.0.0.1:8080",
			EnvVars: []string{"FINCA_ADDR"},
		},
		&cli.StringFlag{
			Name:    "cert",
			Aliases: []string{"c"},
			Usage:   "finca client certificate",
			Value:   "",
		},
		&cli.StringFlag{
			Name:  "key, k",
			Usage: "finca client key",
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
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
