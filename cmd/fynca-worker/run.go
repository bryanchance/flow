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
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fynca/fynca"
	"github.com/fynca/fynca/pkg/tracing"
	"github.com/fynca/fynca/version"
	"github.com/fynca/fynca/worker"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func runAction(clix *cli.Context) error {
	id := clix.String("id")
	configPath := clix.String("config")

	logrus.Infof("starting fynca worker %s", version.BuildVersion())

	cfg, err := fynca.LoadConfig(configPath)
	if err != nil {
		return err
	}

	// check for profiler
	if v := clix.String("profiler-address"); v != "" {
		cfg.ProfilerAddress = v
	}

	// enable tracing if specified
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tp, err := tracing.NewProvider(cfg.TraceEndpoint, "fynca-worker", cfg.Environment)
	if err != nil {
		return err
	}

	w, err := worker.NewWorker(id, cfg)
	if err != nil {
		return err
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	doneCh := make(chan bool, 1)
	errCh := make(chan error, 1)

	var runErr error
	go func() {
		for {
			select {
			case err := <-errCh:
				runErr = err
				doneCh <- true
				return
			case sig := <-signals:
				switch sig {
				case syscall.SIGTERM, syscall.SIGINT:
					logrus.Info("shutting down")
					if err := w.Stop(); err != nil {
						logrus.Error(err)
					}
					doneCh <- true
					return
				default:
					logrus.Warnf("unhandled signal %s", sig)
				}
			}
		}
	}()

	go func() {
		if err := w.Run(); err != nil {
			logrus.WithError(err).Fatal("error starting worker")
		}
		doneCh <- true
	}()

	<-doneCh

	tp.Shutdown(ctx)

	return runErr
}
