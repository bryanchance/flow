package main

import (
	"os"
	"os/signal"
	"syscall"

	"git.underland.io/ehazlett/finca"
	"git.underland.io/ehazlett/finca/version"
	"git.underland.io/ehazlett/finca/worker"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func runAction(clix *cli.Context) error {
	id := clix.String("id")
	configPath := clix.String("config")

	logrus.Infof("starting finca worker %s", version.BuildVersion())

	cfg, err := finca.LoadConfig(configPath)
	if err != nil {
		return err
	}

	w, err := worker.NewWorker(id, cfg)
	if err != nil {
		return err
	}

	if err := w.Run(); err != nil {
		return err
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	doneCh := make(chan bool, 1)

	go func() {
		for {
			select {
			case sig := <-signals:
				switch sig {
				case syscall.SIGTERM, syscall.SIGINT:
					logrus.Info("shutting down")
					if err := w.Stop(); err != nil {
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

	return w.Run()
}
