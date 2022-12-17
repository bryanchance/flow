package main

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

const (
	workflowName = "flow-workflow-webhook"
	workflowType = "dev.ehazlett.flow.webhook"
)

func main() {
	app := cli.NewApp()
	app.Name = workflowName
	app.Usage = "flow webhook workflow"
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
			Usage:   "flow server grpc address",
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
			Usage:   "flow service token for access",
			EnvVars: []string{"SERVICE_TOKEN"},
		},
		&cli.DurationFlag{
			Name:  "workflow-timeout",
			Usage: "workflow processing timeout",
			Value: 8 * time.Hour,
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
