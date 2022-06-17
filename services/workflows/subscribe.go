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
	"sort"
	"sync"
	"time"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workflows/v1"
	"github.com/gogo/protobuf/proto"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type subscriber struct {
	subs map[string]*nats.Subscription
}

func (s *service) SubscribeWorkflowEvents(stream api.Workflows_SubscribeWorkflowEventsServer) error {
	// TODO
	doneCh := make(chan bool)
	errCh := make(chan error)
	workflowLock := &sync.Mutex{}
	maxWorkflows := uint64(0)
	workflowsProcessed := uint64(0)

	go func() {
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				errCh <- err
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
				maxWorkflows = info.MaxWorkflows

				// setup subscriptions
				sub, err := s.newSubscriber(info.ID, info.Type, s.config.GetWorkflowTimeout())
				if err != nil {
					errCh <- err
					return
				}
				defer func() {
					sub.stop()
					close(doneCh)
					close(errCh)
					logrus.Debugf("worker disconnected: %s", info.ID)
				}()

				logrus.Debugf("worker max workflows: %d", info.MaxWorkflows)
				workflowTickerInterval := time.Second * 5
				workflowTicker := time.NewTicker(workflowTickerInterval)

				go func() {
					defer func() {
						workflowLock.Unlock()
						doneCh <- true
					}()

					for range workflowTicker.C {
						workflowLock.Lock()
						if err := s.handleNextMessage(ctx, sub, info, stream); err != nil {
							logrus.WithError(err).Error("error handling next message")
							workflowLock.Unlock()
							continue
						}
					}
				}()
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
						Event: &api.WorkflowEvent_Close{
							Close: true,
						},
					}); err != nil {
						logrus.WithError(err).Errorf("error sending close to", v.Output.ID)
					}
					doneCh <- true
					return
				} else {
					// unlock local stream processing lock
					workflowLock.Unlock()
				}

			default:
				errCh <- fmt.Errorf("unknown request type %s", v)
				return
			}
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-doneCh:
	}

	logrus.Debug("worker finished")

	return nil
}

func (s *service) handleNextMessage(ctx context.Context, sub *subscriber, info *api.SubscriberInfo, stream api.Workflows_SubscribeWorkflowEventsServer) error {
	m, err := sub.nextMessage()
	if err != nil {
		return err
	}
	// if stop has been called the subscription will be drained and closed
	// ignore the subscription error and exit
	if m == nil {
		return nil
	}

	workflow := api.Workflow{}
	if err := proto.Unmarshal(m.Data, &workflow); err != nil {
		return errors.Wrap(err, "error unmarshaling api.Workflow from message")
	}

	logrus.Debugf("received worker workflow %s", workflow.ID)

	// check if job is in job failed cache
	if kv := s.failedWorkflowCache.Get(workflow.ID); kv != nil {
		// TODO: store fail count in datastore to determine max number of failures for a single workflow
		m.Nak()
		return fmt.Errorf("workflow %s is in failed cache; requeueing for another worker", workflow.ID)
	}

	// TODO: check if workflow depends on another workflow output and wait until it is complete

	// report message is in progress
	m.InProgress(nats.AckWait(s.config.GetWorkflowTimeout()))

	// user context for updating workflow to original
	uCtx := context.WithValue(context.Background(), fynca.CtxNamespaceKey, workflow.Namespace)
	workflow.Status = api.WorkflowStatus_RUNNING
	if err := s.ds.UpdateWorkflow(uCtx, &workflow); err != nil {
		m.Nak()
		return err
	}

	logrus.Debugf("processing workflow with timeout %s", s.config.GetWorkflowTimeout())

	logrus.Debugf("sending workflow to subscriber %s", info.ID)
	if err := stream.Send(&api.WorkflowEvent{
		Event: &api.WorkflowEvent_Workflow{
			Workflow: &workflow,
		},
	}); err != nil {
		m.Nak()
		return errors.Wrapf(err, "error sending workflow to processor %s", info.Type)
	}

	return nil
}

func (s *service) updateWorkflowOutput(ctx context.Context, o *api.WorkflowOutput) error {
	// set namespace to workflow for updating
	uCtx := context.WithValue(ctx, fynca.CtxNamespaceKey, o.Namespace)
	w, err := s.ds.GetWorkflow(uCtx, o.ID)
	if err != nil {
		return err
	}

	w.Status = api.WorkflowStatus_COMPLETE
	w.Output = o

	if err := s.ds.UpdateWorkflow(uCtx, w); err != nil {
		return err
	}

	return nil
}

func (s *service) newSubscriber(id, handlerQueueName string, workflowTimeout time.Duration) (*subscriber, error) {
	js, err := s.natsClient.JetStream()
	if err != nil {
		return nil, errors.Wrap(err, "error getting jetstream context")
	}

	subs := map[string]*nats.Subscription{}
	for x, subject := range []string{
		fynca.QueueSubjectJobPriorityUrgent,
		fynca.QueueSubjectJobPriorityNormal,
		fynca.QueueSubjectJobPriorityLow,
	} {
		durableName := fynca.GenerateHash(fmt.Sprintf("%s-%s", handlerQueueName, subject))[:8]
		streamConfig := &nats.StreamConfig{
			Name: fmt.Sprintf("WORKFLOWS_%s", subject),
			Subjects: []string{
				fmt.Sprintf("workflows.%s.>", subject),
			},
			Retention: nats.WorkQueuePolicy,
		}
		logrus.Debugf("creating stream %s using durable name %s: %+v", subject, durableName, streamConfig)
		js.AddStream(streamConfig)

		listenSubject := fmt.Sprintf("workflows.%s.%s", subject, handlerQueueName)
		logrus.Debugf("subscribing to nats subject: %s", listenSubject)
		sub, err := js.PullSubscribe(listenSubject, durableName, nats.AckWait(workflowTimeout))
		if err != nil {
			return nil, errors.Wrap(err, "error subscribing to nats subject")
		}
		subs[fmt.Sprintf("%d-%s", x, subject)] = sub
	}

	return &subscriber{
		subs: subs,
	}, nil
}

// nextMessage returns the next message based on the queue priority order
func (s *subscriber) nextMessage() (*nats.Msg, error) {
	// sort by order
	keys := make([]string, 0)
	for k, _ := range s.subs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, subject := range keys {
		sub := s.subs[subject]
		msgs, err := sub.Fetch(1, nats.MaxWait(1*time.Second))
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
			return nil, err
		}
		logrus.Debugf("received job on subject %s", subject)
		m := msgs[0]
		return m, nil
	}
	return nil, nil
}

func (s *subscriber) stop() {
	for _, sub := range s.subs {
		sub.Unsubscribe()
		sub.Drain()
	}
}
