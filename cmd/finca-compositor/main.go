package main

import (
	"log"
	"os"

	"git.underland.io/ehazlett/finca/version"
	nomadapi "github.com/hashicorp/nomad/api"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "finca-compositor"
	app.Version = version.Version
	app.Authors = []*cli.Author{
		{
			Name: "@ehazlett",
		},
	}
	app.Usage = "finca render slice compositor"
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			EnvVars: []string{"DEBUG"},
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to finca config",
			Value:   "finca.toml",
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

func getNomadClient(clix *cli.Context) (*nomadapi.Client, error) {
	nomadCfg := nomadapi.DefaultConfig()
	nomadCfg.Address = clix.String("nomad-addr")
	if v := clix.String("nomad-region"); v != "" {
		logrus.Debugf("using nomad region %s", v)
		nomadCfg.Region = v
	}
	if v := clix.String("nomad-namespace"); v != "" {
		logrus.Debugf("using nomad ns %s", v)
		nomadCfg.Namespace = v
	}

	return nomadapi.NewClient(nomadCfg)
}

func getMinioClient(clix *cli.Context) (*minio.Client, error) {
	return minio.New(clix.String("s3-endpoint"), &minio.Options{
		Creds:  miniocreds.NewStaticV4(clix.String("s3-access-key-id"), clix.String("s3-access-key-secret"), ""),
		Secure: clix.Bool("s3-use-ssl"),
	})
}
