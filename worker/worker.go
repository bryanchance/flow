package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"git.underland.io/ehazlett/finca/datastore"
	"git.underland.io/ehazlett/finca/pkg/profiler"
	"git.underland.io/ehazlett/finca/version"
	"github.com/dustin/go-humanize"
	"github.com/gogo/protobuf/jsonpb"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type GPU struct {
	Vendor  string
	Product string
}

type Worker struct {
	id         string
	config     *finca.Config
	ds         *datastore.Datastore
	stopCh     chan bool
	cpus       uint32
	memory     int64
	gpus       []*GPU
	gpuEnabled bool
	maxJobs    int
	jobLock    *sync.Mutex
}

func NewWorker(id string, cfg *finca.Config) (*Worker, error) {
	if v := cfg.ProfilerAddress; v != "" {
		p := profiler.NewProfiler(v)
		go func() {
			if err := p.Run(); err != nil {
				logrus.WithError(err).Errorf("error starting profiler on %s", v)
			}
		}()
	}
	cpuThreads, err := getCPUs()
	if err != nil {
		return nil, err
	}

	memory, err := getMemory()
	if err != nil {
		return nil, err
	}

	gpuEnabled, err := gpuEnabled()
	if err != nil {
		return nil, err
	}

	gpus, err := getGPUs()
	if err != nil {
		return nil, err
	}

	ds, err := datastore.NewDatastore(cfg)
	if err != nil {
		return nil, err
	}

	workerConfig := cfg.GetWorkerConfig(id)

	w := &Worker{
		id:         id,
		config:     cfg,
		ds:         ds,
		stopCh:     make(chan bool, 1),
		cpus:       cpuThreads,
		memory:     memory,
		gpus:       gpus,
		gpuEnabled: gpuEnabled,
		maxJobs:    workerConfig.MaxJobs,
		jobLock:    &sync.Mutex{},
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

	// start background listener for worker control
	go w.workerControlListener()

	msgCh := make(chan *nats.Msg, 1)
	doneCh := make(chan bool)

	// connect to nats stream and listen for messages
	logrus.Debugf("monitoring jobs on subject %s", w.config.NATSJobSubject)
	sub, err := js.PullSubscribe(w.config.NATSJobSubject, finca.WorkerQueueGroupName, nats.AckWait(w.config.GetJobTimeout()))
	//sub, err := js.SubscribeSync(w.config.NATSJobSubject, nats.AckWait(w.config.GetJobTimeout()))
	//sub, err := js.ChanQueueSubscribe(w.config.NATSJobSubject, finca.WorkerQueueGroupName, msgCh, nats.AckWait(w.config.GetJobTimeout()))
	if err != nil {
		return errors.Wrapf(err, "error subscribing to nats subject %s", w.config.NATSJobSubject)
	}

	logrus.Debugf("worker max jobs: %d", w.maxJobs)
	jobTickerInterval := time.Second * 5
	jobTicker := time.NewTicker(jobTickerInterval)

	jobsProcessed := 0
	go func() {
		defer func() {
			doneCh <- true
		}()

		for {
			select {
			case <-w.stopCh:
				logrus.Debug("unsubscribing from subject")
				sub.Unsubscribe()

				logrus.Debug("draining")
				sub.Drain()

				close(msgCh)
				return
			case <-jobTicker.C:
				msgs, err := sub.Fetch(1)
				if err != nil {
					// if stop has been called the subscription will be drained and closed
					// ignore the subscription error and exit
					if !sub.IsValid() {
						continue
					}
					if err == nats.ErrTimeout {
						// ignore NextMsg timeouts
						continue
					}
					logrus.Warn(err)
					continue
				}
				m := msgs[0]

				buf := bytes.NewBuffer(m.Data)
				job := &api.Job{}
				if err := jsonpb.Unmarshal(buf, job); err != nil {
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

				workerInfo := w.getWorkerInfo()
				logrus.Debugf("worker info: %+v", workerInfo)
				ctx, _ := context.WithTimeout(context.Background(), w.config.GetJobTimeout())

				job.Worker = workerInfo
				job.Status = api.Job_RENDERING
				job.StartedAt = time.Now()

				if err := w.ds.UpdateJob(ctx, job); err != nil {
					logrus.WithError(err).Error("error updating job")
					// requeue
					m.Nak()
					continue
				}

				j, err := w.processJob(ctx, job)
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
				if err := w.ds.UpdateJob(ctx, j); err != nil {
					logrus.WithError(err).Error("error updating job status")
					continue
				}

				logrus.Infof("completed job %s (%s) (frame: %d slice: %d) in %s", j.ID, j.Request.Name, j.RenderFrame, j.RenderSliceIndex, j.Duration)

				// publish status event for server
				b := &bytes.Buffer{}
				if err := w.ds.Marshaler().Marshal(b, j); err != nil {
					logrus.WithError(err).Error("error publishing job status")
					continue
				}
				js.Publish(w.config.NATSJobStatusSubject, b.Bytes())

				// ack message for completion
				m.Ack()

				// check for max processed jobs
				jobsProcessed += 1
				if w.maxJobs != 0 && jobsProcessed >= w.maxJobs {
					logrus.Infof("worker reached max jobs (%d), exiting", w.maxJobs)
					sub.Unsubscribe()
					sub.Drain()
					close(msgCh)
					return
				}
			}
		}
	}()

	<-doneCh
	logrus.Debug("worker finished")

	return nil
}

func (w *Worker) Stop() error {
	w.jobLock.Lock()
	defer w.jobLock.Unlock()

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
	gpuInfo := []string{}
	for _, gpu := range w.gpus {
		gpuInfo = append(gpuInfo, fmt.Sprintf("%s: %s", gpu.Vendor, gpu.Product))
	}
	return &api.Worker{
		Name:    w.id,
		Version: version.BuildVersion(),
		CPUs:    w.cpus,
		Memory:  w.memory,
		GPUs:    gpuInfo,
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
