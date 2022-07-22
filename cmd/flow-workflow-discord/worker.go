package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

func workerAction(clix *cli.Context) error {
	id := clix.String("id")
	cfg := &workflows.Config{
		ID:                    id,
		Type:                  workflowType,
		Address:               clix.String("address"),
		TLSCertificate:        clix.String("tls-certificate"),
		TLSKey:                clix.String("tls-key"),
		TLSInsecureSkipVerify: clix.Bool("tls-skip-verify"),
		MaxWorkflows:          clix.Uint64("max-workflows"),
		ServiceToken:          clix.String("service-token"),
		Namespace:             clix.String("namespace"),
	}

	p := &Processor{
		discordName:       clix.String("discord-name"),
		discordWebhookURL: clix.String("discord-webhook-url"),
	}

	logrus.Infof("flow processor %s running on %s", workflowType, id)
	h, err := workflows.NewWorkflowHandler(cfg, p)
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
					logrus.Debug("stopping workflow handler")
					if err := h.Stop(); err != nil {
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
		logrus.Debugf("starting workflow handler")
		ctx := context.Background()
		if err := h.Run(ctx); err != nil {
			errCh <- err
			return
		}
		doneCh <- true
	}()

	<-doneCh

	logrus.Info("processor shutting down")

	return runErr
}
