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
	"fmt"
	"io"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/queue"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	errStopReceived = errors.New("stop was called")
)

type subscriber struct {
	subs map[string]*nats.Subscription
}

func (s *service) SubscribeWorkflowEvents(stream api.Workflows_SubscribeWorkflowEventsServer) error {
	doneCh := make(chan bool)
	stopCh := make(chan bool)
	errCh := make(chan error)
	maxWorkflows := uint64(0)
	workflowsProcessed := uint64(0)

	workflowTickerInterval := time.Second * 5
	workflowTicker := time.NewTicker(workflowTickerInterval)
	defer workflowTicker.Stop()

	workflowQueue, err := queue.NewQueue(s.config.QueueAddress)
	if err != nil {
		return err
	}

	go func() {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				errCh <- err
				return
			}

			ctx := stream.Context()
			switch v := req.GetRequest().(type) {
			case *api.SubscribeWorkflowEventsRequest_Info:
				info := v.Info
				logrus.Debugf("received workflow subscriber request: %+v", info)

				if err := s.registerProcessor(info); err != nil {
					if err := stream.Send(&api.WorkflowEvent{
						Event: &api.WorkflowEvent_Close{
							Close: &api.WorkflowCloseEvent{
								Error: &api.WorkflowError{
									Error: err.Error(),
								},
							},
						},
					}); err != nil {
						logrus.WithError(err).Errorf("error registering process %s", info.ID)
					}
					doneCh <- true
					return
				}

				maxWorkflows = info.MaxWorkflows

				logrus.Debugf("worker max workflows: %d", info.MaxWorkflows)

				go func() {
					defer func() {
						s.unRegisterProcessor(info)
						doneCh <- true
					}()

					for {
						select {
						case <-workflowTicker.C:
							if err := s.handleNextMessage(ctx, workflowQueue, info, stream); err != nil {
								logrus.WithError(err).Error("error handling next message")
								return
							}
						case <-stopCh:
							logrus.Infof("worker disconnected: %s", info.ID)
							return
						}
					}
				}()
			case *api.SubscribeWorkflowEventsRequest_Ack:
				ack := v.Ack
				logrus.Debugf("processor ACK'd workflow %s", ack.ID)
				uCtx := context.WithValue(context.Background(), flow.CtxNamespaceKey, ack.Namespace)
				workflow, err := s.ds.GetWorkflow(uCtx, ack.ID)
				if err != nil {
					errCh <- err
					return
				}

				workflow.Status = ack.Status
				if err := s.ds.UpdateWorkflow(uCtx, workflow); err != nil {
					errCh <- err
					return
				}
			case *api.SubscribeWorkflowEventsRequest_Output:
				logrus.Debugf("received workflow output: %+v", v.Output)

				if err := s.updateWorkflowOutput(ctx, v.Output); err != nil {
					logrus.WithError(err).Errorf("error updating workflow output for %s", v.Output.ID)
				}

				// check for max processed jobs
				workflowsProcessed += 1
				if maxWorkflows != 0 && workflowsProcessed >= maxWorkflows {
					logrus.Infof("worker reached max workflows (%d), exiting", maxWorkflows)
					if err := stream.Send(&api.WorkflowEvent{
						Event: &api.WorkflowEvent_Close{},
					}); err != nil {
						logrus.WithError(err).Errorf("error sending close to", v.Output.ID)
					}
					doneCh <- true
					return
				}

			default:
				errCh <- fmt.Errorf("unknown request type %s", v)
				return
			}
		}
	}()

	select {
	case err := <-errCh:
		stopCh <- true
		return err
	case <-doneCh:
	}

	return nil
}

func (s *service) handleNextMessage(ctx context.Context, q *queue.Queue, info *api.ProcessorInfo, stream api.Workflows_SubscribeWorkflowEventsServer) error {
	task, err := q.Pull(ctx, info.Type)
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}

	ns, id, err := parseWorkflowQueueValue(task.Data)
	if err != nil {
		return err
	}

	uCtx := context.WithValue(ctx, flow.CtxNamespaceKey, ns)
	workflow, err := s.ds.GetWorkflow(uCtx, id)
	if err != nil {
		return err
	}

	logrus.Debugf("received worker workflow %s", workflow.ID)

	// check if job is in job failed cache
	if kv := s.failedWorkflowCache.Get(workflow.ID); kv != nil {
		// TODO: store fail count in datastore to determine max number of failures for a single workflow
		// requeue
		v := getWorkflowQueueValue(workflow)
		if err := q.Schedule(ctx, info.Type, v, task.Priority); err != nil {
			return err
		}
		return fmt.Errorf("workflow %s is in failed cache; requeueing for another worker", workflow.ID)
	}

	// TODO: check if workflow depends on another workflow output and wait until it is complete

	// user context for updating workflow to original
	logrus.Debugf("processing workflow with timeout %s", s.config.GetWorkflowTimeout())

	logrus.Debugf("sending workflow to subscriber %s", info.ID)
	if err := stream.Send(&api.WorkflowEvent{
		Event: &api.WorkflowEvent_Workflow{
			Workflow: workflow,
		},
	}); err != nil {
		// requeue
		v := getWorkflowQueueValue(workflow)
		if err := q.Schedule(ctx, info.Type, v, task.Priority); err != nil {
			return err
		}
		return errors.Wrapf(err, "error sending workflow to processor %s", info.Type)
	}

	return nil
}

func (s *service) registerProcessor(p *api.ProcessorInfo) error {
	s.processorLock.Lock()
	defer s.processorLock.Unlock()

	processorID := flow.GenerateHash(p.ID, p.Type)
	if v, ok := s.processors[processorID]; ok {
		return fmt.Errorf("processor with that ID already registered for that type: %s", v.ID)
	}

	logrus.Debugf("adding %s (%s) to processors", p.ID, p.Type)
	p.StartedAt = time.Now()
	s.processors[processorID] = p
	return nil
}

func (s *service) unRegisterProcessor(p *api.ProcessorInfo) {
	s.processorLock.Lock()
	defer s.processorLock.Unlock()

	processorID := flow.GenerateHash(p.ID, p.Type)
	if p, ok := s.processors[processorID]; ok {
		logrus.Debugf("removing %s (%s) from processors", p.ID, p.Type)
		delete(s.processors, processorID)
	}
}

func (s *service) updateWorkflowOutput(ctx context.Context, o *api.WorkflowOutput) error {
	// set namespace to workflow for updating
	uCtx := context.WithValue(ctx, flow.CtxNamespaceKey, o.Namespace)
	w, err := s.ds.GetWorkflow(uCtx, o.ID)
	if err != nil {
		return err
	}

	w.Output = o

	if err := s.ds.UpdateWorkflow(uCtx, w); err != nil {
		return err
	}

	return nil
}
