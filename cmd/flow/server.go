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
	"os"
	"os/signal"
	"syscall"

	"github.com/ehazlett/flow"
	"github.com/ehazlett/flow/server"
	"github.com/ehazlett/flow/services"
	accountsservice "github.com/ehazlett/flow/services/accounts"
	infoservice "github.com/ehazlett/flow/services/info"
	workflowsservice "github.com/ehazlett/flow/services/workflows"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var serverCommand = &cli.Command{
	Name:  "server",
	Usage: "start flow server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to flow config",
			Value:   "flow.toml",
		},
	},
	Action: serverAction,
}

func serverAction(clix *cli.Context) error {
	cfg, err := flow.LoadConfig(clix.String("config"))
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

	svcs := []func(cfg *flow.Config) (services.Service, error){
		infoservice.New,
		workflowsservice.New,
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
