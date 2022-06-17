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
package workflows

import (
	"context"
	"sync"
	"time"

	"github.com/ehazlett/ttlcache"
	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workflows/v1"
	"github.com/fynca/fynca/datastore"
	"github.com/gogo/protobuf/proto"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

const (
	// time to wait after a failed job to retry the workflow again
	workflowFailedCacheDelay = time.Second * 60
)

type WorkflowHandler struct {
	id                  string
	handlerType         string
	config              *fynca.Config
	ds                  *datastore.Datastore
	maxWorkflows        int
	workflowLock        *sync.Mutex
	workflowTimeout     time.Duration
	failedWorkflowCache *ttlcache.TTLCache
	stopCh              chan bool
	paused              bool
	processor           Processor
}

func NewWorkflowHandler(id string, handlerType string, workflowTimeout time.Duration, maxWorkflows int, cfg *fynca.Config, p Processor) (*WorkflowHandler, error) {
	ds, err := datastore.NewDatastore(cfg)
	if err != nil {
		return nil, err
	}

	failedWorkflowCache, err := ttlcache.NewTTLCache(workflowFailedCacheDelay)
	if err != nil {
		return nil, err
	}

	return &WorkflowHandler{
		id:                  id,
		handlerType:         handlerType,
		config:              cfg,
		workflowLock:        &sync.Mutex{},
		workflowTimeout:     workflowTimeout,
		ds:                  ds,
		maxWorkflows:        maxWorkflows,
		failedWorkflowCache: failedWorkflowCache,
		stopCh:              make(chan bool, 1),
		processor:           p,
	}, nil
}

func (h *WorkflowHandler) Run() error {
	// setup subscriptions
	sub, err := newSubscriber(h.id, h.handlerType, h.config, h.workflowTimeout)
	if err != nil {
		return err
	}
	logrus.Debugf("worker max workflows: %d", h.maxWorkflows)
	workflowTickerInterval := time.Second * 5
	workflowTicker := time.NewTicker(workflowTickerInterval)

	msgCh := make(chan *nats.Msg, 1)
	doneCh := make(chan bool)

	workflowsProcessed := 0
	go func() {
		defer func() {
			doneCh <- true
		}()

		for {
			select {
			case <-h.stopCh:
				logrus.Debug("stopCh")
				sub.stop()
				close(msgCh)
				return
			case <-workflowTicker.C:
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

				workflow := api.Workflow{}
				if err := proto.Unmarshal(m.Data, &workflow); err != nil {
					logrus.WithError(err).Error("error unmarshaling api.Workflow from message")
					continue
				}
				logrus.Debugf("received worker workflow %s", workflow.ID)

				// check if job is in job failed cache
				if kv := h.failedWorkflowCache.Get(workflow.ID); kv != nil {
					// TODO: store fail count in datastore to determine max number of failures for a single workflow
					logrus.Warnf("workflow %s is in failed cache; requeueing for another worker", workflow.ID)
					m.Nak()
					continue
				}

				// TODO: check if workflow depends on another workflow output and wait until it is complete

				// report message is in progress
				m.InProgress(nats.AckWait(h.workflowTimeout))
				uCtx := context.WithValue(context.Background(), fynca.CtxNamespaceKey, workflow.Namespace)
				// user context for updating workflow to original
				workflow.Status = api.WorkflowStatus_RUNNING
				if err := h.ds.UpdateWorkflow(uCtx, &workflow); err != nil {
					m.Nak()
					continue
				}

				logrus.Debugf("processing workflow with timeout %s", h.workflowTimeout)

				ctx, cancel := context.WithTimeout(context.Background(), h.workflowTimeout)
				output, processErr := h.processor.Process(ctx, &workflow)
				if processErr != nil {
					if err := h.failedWorkflowCache.Set(workflow.ID, true); err != nil {
						logrus.WithError(err).Errorf("error setting workflow failed cache for %s", workflow.ID)
					}
					workflow.Output = &api.WorkflowOutput{
						Log: processErr.Error(),
					}
					workflow.Status = api.WorkflowStatus_ERROR
					m.Nak()
					cancel()
				} else {
					// assign output to workflow
					workflow.Output = output
					workflow.Status = api.WorkflowStatus_COMPLETE
				}

				// update workflow in db
				if err := h.ds.UpdateWorkflow(uCtx, &workflow); err != nil {
					m.Nak()
					logrus.WithError(err).Errorf("error updating workflow for %s in datastore", workflow.ID)
				} else {
					// ACK message
					m.Ack()
				}
				cancel()

				// check for max processed jobs
				workflowsProcessed += 1
				if h.maxWorkflows != 0 && workflowsProcessed >= h.maxWorkflows {
					logrus.Infof("worker reached max workflows (%d), exiting", h.maxWorkflows)
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

func (h *WorkflowHandler) Stop() error {
	h.workflowLock.Lock()
	defer h.workflowLock.Unlock()

	h.stopCh <- true
	logrus.Debug("stop finished")
	return nil
}

func (h *WorkflowHandler) getMinioClient() (*minio.Client, error) {
	return minio.New(h.config.S3Endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(h.config.S3AccessID, h.config.S3AccessKey, ""),
		Secure: h.config.S3UseSSL,
	})
}
