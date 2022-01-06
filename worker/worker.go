package worker

import (
	"context"
	"encoding/json"
	"fmt"
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
	sub, err := js.PullSubscribe(w.config.NATSSubject+".*", finca.WorkerQueueGroupName,
		nats.MaxDeliver(3),
	)
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

				//m.Ack()
				m.InProgress(nats.AckWait(w.config.GetJobTimeout()))

				logrus.Debugf("processing job with timeout %s", w.config.GetJobTimeout())
				start := time.Now()
				ctx, _ := context.WithTimeout(context.Background(), w.config.GetJobTimeout())
				if err := w.processJob(ctx, job); err != nil {
					logrus.WithError(err).Errorf("error processing job %s", job.ID)
					if cErr := ctx.Err(); cErr != nil {
						logrus.Warnf("job timeout occurred during processing for %s", job.ID)
					} else {
						// no timeout; requeue job
						logrus.Warnf("requeueing job %s", job.ID)
						m.Nak()
					}
					continue
				}
				jobID := fmt.Sprintf("%s (frame: %04d)", job.ID, job.RenderFrame)
				if job.Request.RenderSlices > 0 {
					jobID = fmt.Sprintf("%s (frame: %04d slice: %0.2f %0.2f %0.2f %0.2f)",
						job.ID,
						job.RenderFrame,
						job.RenderSliceMinX,
						job.RenderSliceMaxX,
						job.RenderSliceMinY,
						job.RenderSliceMaxY,
					)
				}
				logrus.Infof("completed job %s in %s", jobID, time.Now().Sub(start))

				m.Ack()
			}
		}
	}()

	go func() {
		for {
			msgs, err := sub.Fetch(1)
			if err != nil {
				// if stop has been called the subscription will be drained and closed
				// ignore the subscription error and exit
				if !sub.IsValid() {
					return
				}
				if err == nats.ErrTimeout {
					// ignore NextMsg timeouts
					continue
				}
				logrus.Warn(err)
				return
			}

			msgCh <- msgs[0]
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
