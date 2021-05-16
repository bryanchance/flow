package main

import (
	"fmt"
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
			Name:    "nomad-addr",
			Usage:   "nomad api address",
			EnvVars: []string{"NOMAD_ADDR"},
		},
		&cli.StringFlag{
			Name:    "nomad-region",
			Usage:   "nomad region",
			EnvVars: []string{"NOMAD_REGION"},
			Value:   "global",
		},
		&cli.StringFlag{
			Name:    "nomad-namespace",
			Usage:   "nomad namespace",
			EnvVars: []string{"NOMAD_NAMESPACE"},
			Value:   "default",
		},
		&cli.StringFlag{
			Name:    "nomad-job-id",
			Usage:   "nomad job ID for processing",
			EnvVars: []string{"NOMAD_JOB_ID"},
		},
		&cli.IntFlag{
			Name:    "render-slices",
			Usage:   "number of render slices to expect for processing",
			EnvVars: []string{"RENDER_SLICES"},
			Value:   1,
		},
		&cli.IntFlag{
			Name:    "render-start-frame",
			Usage:   "render start frame",
			EnvVars: []string{"RENDER_START_FRAME"},
			Value:   1,
		},
		&cli.IntFlag{
			Name:    "render-end-frame",
			Usage:   "render end frame",
			EnvVars: []string{"RENDER_END_FRAME"},
			Value:   1,
		},
		&cli.StringFlag{
			Name:    "project-name",
			Usage:   "project name",
			EnvVars: []string{"PROJECT_NAME"},
		},
		&cli.StringFlag{
			Name:    "s3-access-key-id",
			Usage:   "s3 access key id",
			EnvVars: []string{"S3_ACCESS_KEY_ID"},
		},
		&cli.StringFlag{
			Name:    "s3-access-key-secret",
			Usage:   "s3 access secret",
			EnvVars: []string{"S3_ACCESS_KEY_SECRET"},
		},
		&cli.BoolFlag{
			Name:    "s3-use-ssl",
			Usage:   "enable ssl for s3",
			EnvVars: []string{"S3_USE_SSL"},
		},
		&cli.StringFlag{
			Name:    "s3-endpoint",
			Usage:   "s3 endpoint for processing files",
			EnvVars: []string{"S3_ENDPOINT"},
		},
		&cli.StringFlag{
			Name:    "s3-bucket",
			Usage:   "s3 bucket for processing files",
			EnvVars: []string{"S3_BUCKET"},
		},
		&cli.StringFlag{
			Name:    "s3-render-directory",
			Usage:   "directory for finca render targets",
			EnvVars: []string{"S3_RENDER_DIRECTORY"},
		},
	}
	app.Before = func(clix *cli.Context) error {
		if clix.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		switch {
		case clix.String("nomad-job-id") == "":
			cli.ShowAppHelp(clix)
			return fmt.Errorf("nomad-job-id cannot be empty")
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
