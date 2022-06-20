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
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ehazlett/flow"
	jobapi "github.com/ehazlett/flow/api/services/jobs/v1"
	workersapi "github.com/ehazlett/flow/api/services/workers/v1"
	"github.com/ehazlett/flow/datastore"
	"github.com/ehazlett/flow/pkg/profiler"
	"github.com/ehazlett/flow/version"
	"github.com/dustin/go-humanize"
	"github.com/ehazlett/ttlcache"
	"github.com/gogo/protobuf/proto"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
)

const (
	serviceName = "worker"
)

var (
	// ERRGPUNotSupported is returned when the worker does not support GPU workloads
	ErrGPUNotSupported = errors.New("worker does not support GPU workloads")

	// time to wait after a failed job to retry the job again
	jobFailedCacheDelay = time.Second * 60
)

type GPU struct {
	Vendor  string
	Product string
}

type Worker struct {
	id             string
	config         *fynca.Config
	natsClient     *nats.Conn
	ds             *datastore.Datastore
	stopCh         chan bool
	gpus           []*GPU
	gpuEnabled     bool
	maxJobs        int
	jobLock        *sync.Mutex
	failedJobCache *ttlcache.TTLCache
	paused         bool
}

func NewWorker(id string, cfg *fynca.Config) (*Worker, error) {
	if v := cfg.ProfilerAddress; v != "" {
		p := profiler.NewProfiler(v)
		go func() {
			if err := p.Run(); err != nil {
				logrus.WithError(err).Errorf("error starting profiler on %s", v)
			}
		}()
	}
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to nats")
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

	failedJobCache, err := ttlcache.NewTTLCache(jobFailedCacheDelay)
	if err != nil {
		return nil, err
	}

	w := &Worker{
		id:             id,
		config:         cfg,
		natsClient:     nc,
		ds:             ds,
		stopCh:         make(chan bool, 1),
		gpus:           gpus,
		gpuEnabled:     gpuEnabled,
		maxJobs:        workerConfig.MaxJobs,
		jobLock:        &sync.Mutex{},
		failedJobCache: failedJobCache,
		paused:         false,
	}

	return w, nil
}

func (w *Worker) Run() error {
	if err := w.showWorkerInfo(); err != nil {
		return err
	}
	logrus.Infof("GPU enabled: %v", w.gpuEnabled)

	logrus.Debugf("connecting to nats: %s", w.config.NATSURL)

	js, err := w.natsClient.JetStream()
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
	logrus.Debugf("monitoring jobs on stream %s", w.config.NATSJobStreamName)

	// setup subscriptions
	sub, err := newSubscriber(w.config)
	if err != nil {
		return err
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
				sub.stop()
				close(msgCh)
				return
			case <-jobTicker.C:
				m, err := sub.nextMessage()
				if err != nil {
					logrus.Warn(err)
					continue
				}
				// if stop has been called the subscription will be drained and closed
				// ignore the subscription error and exit
				if m == nil {
					continue
				}

				// check if paused
				if w.paused {
					logrus.Info("worker is paused; requeueing for another worker")
					m.Nak()
					continue
				}

				workerJob := jobapi.WorkerJob{}
				if err := proto.Unmarshal(m.Data, &workerJob); err != nil {
					logrus.WithError(err).Error("error unmarshaling jobapi.WorkerJob from message")
					continue
				}
				logrus.Debugf("received worker job %s", workerJob.ID)
				jobID := workerJob.ID

				// check if job is in job failed cache
				if kv := w.failedJobCache.Get(jobID); kv != nil {
					logrus.Warnf("job %s is in failed job cache; requeueing for another worker", jobID)
					m.Nak()
					continue
				}

				// report message is in progress
				m.InProgress(nats.AckWait(w.config.GetJobTimeout()))

				logrus.Debugf("processing job with timeout %s", w.config.GetJobTimeout())

				workerInfo, err := w.getWorkerInfo()
				if err != nil {
					logrus.WithError(err).Error("error getting worker info")
					continue
				}
				logrus.Debugf("worker info: %+v", workerInfo)

				ctx, cancel := context.WithTimeout(context.Background(), w.config.GetJobTimeout())

				result := &jobapi.JobResult{}

				var pErr error
				switch v := workerJob.GetJob().(type) {
				case *jobapi.WorkerJob_FrameJob:
					pCtx := context.WithValue(ctx, fynca.CtxNamespaceKey, v.FrameJob.Request.Namespace)
					result, pErr = w.processFrameJob(pCtx, v.FrameJob)
				case *jobapi.WorkerJob_SliceJob:
					pCtx := context.WithValue(ctx, fynca.CtxNamespaceKey, v.SliceJob.Request.Namespace)
					result, pErr = w.processSliceJob(pCtx, v.SliceJob)
				default:
					logrus.Errorf("unknown job type %+T", v)
					cancel()
					continue
				}

				if pErr != nil {
					if pErr == ErrGPUNotSupported {
						m.Nak()
						cancel()
						continue
					}

					// publish error
					logrus.WithError(pErr).Errorf("error processing job")
					if cErr := ctx.Err(); cErr != nil {
						logrus.Warn("job timeout occurred during processing")
					} else {
						// no timeout; requeue job
						logrus.WithError(pErr).Error("error occurred while processing job")
						if result == nil {
							result = &jobapi.JobResult{}
						}
						result.Status = jobapi.JobStatus_ERROR
						result.Error = pErr.Error()
						cancel()
					}
				}

				logrus.Infof("completed job %s: status=%s", jobID, result.Status)

				// publish status event for server
				data, err := proto.Marshal(result)
				if err != nil {
					logrus.WithError(err).Error("error publishing job result")
					continue
				}
				js.Publish(w.config.NATSJobStatusStreamName, data)

				// ack message for completion
				if result.Status == jobapi.JobStatus_ERROR {
					logrus.WithError(pErr).Infof("error processing job; requeueing %s", jobID)
					// add to failed job cache to prevent reprocessing immediately
					if err := w.failedJobCache.Set(jobID, true); err != nil {
						logrus.WithError(err).Errorf("error setting job failed cache for %s", jobID)
					}
					// requeue
					m.Nak()
				} else {
					// success
					m.Ack()
				}

				// check for max processed jobs
				jobsProcessed += 1
				if w.maxJobs != 0 && jobsProcessed >= w.maxJobs {
					logrus.Infof("worker reached max jobs (%d), exiting", w.maxJobs)
					sub.stop()
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

func (w *Worker) getWorkerInfo() (*workersapi.Worker, error) {
	gpuInfo := []string{}
	for _, gpu := range w.gpus {
		gpuInfo = append(gpuInfo, fmt.Sprintf("%s: %s", gpu.Vendor, gpu.Product))
	}

	cpus, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	loadStats, err := load.Avg()
	if err != nil {
		return nil, err
	}

	m, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	return &workersapi.Worker{
		Name:            w.id,
		Version:         version.BuildVersion(),
		CPUs:            uint32(cpus),
		MemoryTotal:     int64(m.Total),
		MemoryAvailable: int64(m.Available),
		GPUs:            gpuInfo,
		Load1:           loadStats.Load1,
		Load5:           loadStats.Load5,
		Load15:          loadStats.Load15,
		Paused:          w.paused,
	}, nil
}

func (w *Worker) showWorkerInfo() error {
	info, err := w.getWorkerInfo()
	if err != nil {
		return err
	}

	logrus.Infof("CPU: %d", info.CPUs)
	logrus.Infof("Memory: %s", humanize.Bytes(uint64(info.MemoryTotal)))

	for _, gpu := range w.gpus {
		logrus.Infof("Graphics Card: %s", gpu)
	}

	return nil
}

func (w *Worker) workerHeartbeat() {
	t := time.NewTicker(fynca.WorkerTTL)
	for range t.C {
		workerInfo, err := w.getWorkerInfo()
		if err != nil {
			logrus.WithError(err).Error("error getting worker info")
			continue
		}

		ctx := context.Background()
		if err := w.ds.UpdateWorkerInfo(ctx, workerInfo); err != nil {
			logrus.WithError(err).Error("error updating worker info")
			continue
		}
	}
}
