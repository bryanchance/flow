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

	"github.com/fynca/fynca"
	"github.com/fynca/fynca/pkg/workflows"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func workerAction(clix *cli.Context) error {
	configPath := clix.String("config")
	cfg, err := fynca.LoadConfig(configPath)
	if err != nil {
		return err
	}

	id := clix.String("id")
	taskTimeout := clix.Duration("task-timeout")
	maxTasks := clix.Int("max-tasks")

	p := &Processor{}
	handler, err := workflows.NewWorkflowHandler(id, workflowType, taskTimeout, maxTasks, cfg, p)
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
					if err := handler.Stop(); err != nil {
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
		if err := handler.Run(); err != nil {
			errCh <- err
			return
		}
		doneCh <- true
	}()

	<-doneCh

	return runErr
}
