package worker

import (
	"context"
	"encoding/json"
	"time"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Worker struct {
	id     string
	config *finca.Config
	stopCh chan bool
}

func NewWorker(id string, cfg *finca.Config) (*Worker, error) {
	return &Worker{
		id:     id,
		config: cfg,
		stopCh: make(chan bool, 1),
	}, nil
}

func (w *Worker) Run() error {
	logrus.Debugf("connecting to nats: %s", w.config.NATSURL)
	nc, err := nats.Connect(w.config.NATSURL)
	if err != nil {
		return errors.Wrap(err, "error connecting to nats")
	}
	js, err := nc.JetStream()
	if err != nil {
		return errors.Wrap(err, "error getting jetstream context")
	}

	msgCh := make(chan *nats.Msg)

	// connect to nats stream and listen for messages
	logrus.Debugf("monitoring jobs on subject %s", w.config.NATSSubject)
	sub, err := js.ChanSubscribe(w.config.NATSSubject+".*", msgCh, nats.Durable(w.id))
	if err != nil {
		return errors.Wrapf(err, "error subscribing to nats subject %s", w.config.NATSSubject)
	}

	go func() {
		for {
			select {
			case <-w.stopCh:
				logrus.Debug("unsubscribing from subject")
				sub.Unsubscribe()

				logrus.Debug("draining")
				sub.Drain()

				close(msgCh)
				return
			case m := <-msgCh:
				var job *api.Job
				if err := json.Unmarshal(m.Data, &job); err != nil {
					logrus.WithError(err).Error("error unmarshaling api.Job from message")
					continue
				}
				m.Ack()
				logrus.Debugf("processing job with timeout %s", w.config.GetJobTimeout())
				start := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), w.config.GetJobTimeout())
				if err := w.processJob(ctx, job); err != nil {
					logrus.WithError(err).Errorf("error processing job %s", job.ID)
					cancel()
					continue
				}
				logrus.Infof("processed job %s in %s", job.ID, time.Now().Sub(start))
				cancel()
			}
		}
	}()

	return nil
}

func (w *Worker) Stop() error {
	w.stopCh <- true
	return nil
}

func (w *Worker) getMinioClient() (*minio.Client, error) {
	return minio.New(w.config.S3Endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(w.config.S3AccessID, w.config.S3AccessKey, ""),
		Secure: w.config.S3UseSSL,
	})
}
