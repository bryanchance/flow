package main

import (
	"log"
	"os"

	"git.underland.io/ehazlett/fynca/version"
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
