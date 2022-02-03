package main

import (
	"os"
	"os/signal"
	"syscall"

	"git.underland.io/ehazlett/fynca"
	"git.underland.io/ehazlett/fynca/server"
	"git.underland.io/ehazlett/fynca/services"
	accountsservice "git.underland.io/ehazlett/fynca/services/accounts"
	renderservice "git.underland.io/ehazlett/fynca/services/render"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var serverCommand = &cli.Command{
	Name:  "server",
	Usage: "start fynca server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to fynca config",
			Value:   "fynca.toml",
		},
	},
	Action: serverAction,
}

func serverAction(clix *cli.Context) error {
	cfg, err := fynca.LoadConfig(clix.String("config"))
	if err != nil {
		return err
	}

	// check for profiler
	if v := clix.String("profiler-address"); v != "" {
		cfg.ProfilerAddress = v
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		return err
	}

	svcs := []func(cfg *fynca.Config) (services.Service, error){
		renderservice.New,
	}

	if cfg.Authenticator != nil && cfg.Authenticator.Name != "none" {
		svcs = append(svcs, accountsservice.New)
	}

	if err := srv.Register(svcs); err != nil {
		return err
	}

	if err := srv.Run(); err != nil {
		return err
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	doneCh := make(chan bool, 1)

	go func() {
		for {
			select {
			case sig := <-signals:
				switch sig {
				case syscall.SIGUSR1:
					logrus.Debug("generating debug profile")
					profilePath, err := srv.GenerateProfile()
					if err != nil {
						logrus.Error(err)
						continue
					}
					logrus.WithFields(logrus.Fields{
						"profile": profilePath,
					}).Info("generated memory profile")
				case syscall.SIGTERM, syscall.SIGINT:
					logrus.Info("shutting down")
					if err := srv.Stop(); err != nil {
						logrus.Error(err)
					}
					doneCh <- true
				default:
					logrus.Warnf("unhandled signal %s", sig)
				}
			}
		}
	}()

	<-doneCh

	return nil
}
