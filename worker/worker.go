package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"git.underland.io/ehazlett/finca/version"
	"github.com/dustin/go-humanize"
	"github.com/jaypipes/ghw"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Worker struct {
	id         string
	config     *finca.Config
	stopCh     chan bool
	cpus       uint32
	memory     int64
	gpus       []string
	gpuEnabled bool
}

func NewWorker(id string, cfg *finca.Config) (*Worker, error) {
	cpu, err := ghw.CPU()
	if err != nil {
		return nil, err
	}

	mem, err := ghw.Memory()
	if err != nil {
		return nil, err
	}

	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}

	gpuEnabled, err := gpuEnabled()
	if err != nil {
		return nil, err
	}

	gpus := []string{}
	for _, card := range gpu.GraphicsCards {
		gpus = append(gpus, fmt.Sprintf("%s: %s", card.DeviceInfo.Vendor.Name, card.DeviceInfo.Product.Name))
	}

	w := &Worker{
		id:         id,
		config:     cfg,
		stopCh:     make(chan bool, 1),
		cpus:       cpu.TotalThreads,
		memory:     mem.TotalUsableBytes,
		gpus:       gpus,
		gpuEnabled: gpuEnabled,
	}

	return w, nil
}

func (w *Worker) Run() error {
	if err := w.showHardwareInfo(); err != nil {
		return err
	}
	logrus.Infof("GPU enabled: %v", w.gpuEnabled)

	logrus.Debugf("connecting to nats: %s", w.config.NATSURL)

	nc, err := nats.Connect(w.config.NATSURL)
	if err != nil {
		return errors.Wrap(err, "error connecting to nats")
	}
	js, err := nc.JetStream()
	if err != nil {
		return errors.Wrap(err, "error getting jetstream context")
	}

	// start worker heartbeat
	go w.workerHeartbeat()

	msgCh := make(chan *nats.Msg)

	// connect to nats stream and listen for messages
	logrus.Debugf("monitoring jobs on subject %s", w.config.NATSJobSubject)
	sub, err := js.PullSubscribe(w.config.NATSJobSubject, finca.WorkerQueueGroupName, nats.AckWait(w.config.GetJobTimeout()))
	if err != nil {
		return errors.Wrapf(err, "error subscribing to nats subject %s", w.config.NATSJobSubject)
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

				// check for gpu
				if job.Request.RenderUseGPU && !w.gpuEnabled {
					logrus.Debug("skipping GPU job as worker does not have GPU")
					m.Nak()
					continue
				}

				// report message is in progress
				m.InProgress(nats.AckWait(w.config.GetJobTimeout()))

				logrus.Debugf("processing job with timeout %s", w.config.GetJobTimeout())

				ctx, _ := context.WithTimeout(context.Background(), w.config.GetJobTimeout())
				jobStatus, err := w.processJob(ctx, job)
				if err != nil {
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

				logrus.Infof("completed job %s (frame: %d slice: %d) in %s", job.ID, job.RenderFrame, job.RenderSliceIndex, jobStatus.Duration)

				// publish status event for server
				jobStatusData, err := json.Marshal(jobStatus)
				if err != nil {
					logrus.WithError(err).Error("error publishing job status")
					continue
				}
				js.Publish(w.config.NATSJobStatusSubject, jobStatusData)

				// ack message for completion
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

func (w *Worker) getWorkerInfo() *api.Worker {
	return &api.Worker{
		Name:    w.id,
		Version: version.BuildVersion(),
		CPUs:    w.cpus,
		Memory:  w.memory,
		GPUs:    w.gpus,
	}
}
func (w *Worker) showHardwareInfo() error {
	logrus.Infof("CPU: %d", w.cpus)
	logrus.Infof("Memory: %s", humanize.Bytes(uint64(w.memory)))

	for _, gpu := range w.gpus {
		logrus.Infof("Graphics Card: %s", gpu)
	}

	return nil
}

func (w *Worker) workerHeartbeat() {
	nc, err := nats.Connect(w.config.NATSURL)
	if err != nil {
		logrus.Fatalf("error connecting to nats on %s", w.config.NATSURL)
	}

	js, err := nc.JetStream()
	if err != nil {
		logrus.Fatal("error getting stream context")
	}

	kv, err := js.KeyValue(w.config.NATSKVBucketNameWorkers)
	if err != nil {
		logrus.Fatalf("error getting kv %s from nats", w.config.NATSKVBucketNameWorkers)
	}

	t := time.NewTicker(finca.KVBucketTTLWorkers)
	for range t.C {
		workerInfo := w.getWorkerInfo()
		workerData, err := json.Marshal(workerInfo)
		if err != nil {
			logrus.WithError(err).Error("error getting worker info")
			continue
		}
		if _, err := kv.Put(w.id, workerData); err != nil {
			logrus.WithError(err).Error("error updating worker heartbeat info")
			continue
		}
	}
}

func gpuEnabled() (bool, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return false, err
	}

	for _, card := range gpu.GraphicsCards {
		if strings.Index(strings.ToLower(strings.TrimSpace(card.DeviceInfo.Vendor.Name)), "nvidia") > -1 {
			return true, nil
		}
	}

	return false, nil
}
