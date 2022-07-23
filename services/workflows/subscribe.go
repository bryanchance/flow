package workflows

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	errStopReceived = errors.New("stop was called")
)

type subscriber struct {
}

func (s *service) SubscribeWorkflowEvents(stream api.Workflows_SubscribeWorkflowEventsServer) error {
	doneCh := make(chan bool)
	stopCh := make(chan bool)
	errCh := make(chan error)

	req, err := stream.Recv()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}

	ctx := stream.Context()
	pCtx, cancel := context.WithCancel(ctx)
	defer cancel()

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
			return err
		}

		logrus.Debugf("worker max workflows: %d", info.MaxWorkflows)

		go func() {
			defer func() {
				s.unRegisterProcessor(info)
				doneCh <- true
			}()

			if err := s.processQueue(pCtx, info, stream, info.MaxWorkflows); err != nil {
				errCh <- err
				return
			}
		}()
	}

	select {
	case err := <-errCh:
		stopCh <- true
		cancel()
		return err
	case <-doneCh:
	}

	return nil
}

func (s *service) processQueue(ctx context.Context, info *api.ProcessorInfo, stream api.Workflows_SubscribeWorkflowEventsServer, maxWorkflows uint64) error {
	if info.Scope == nil || info.Scope.Scope == nil {
		return fmt.Errorf("Scope not defined")
	}

	workflowTickerInterval := time.Second * 5
	workflowTicker := time.NewTicker(workflowTickerInterval)
	defer workflowTicker.Stop()

	errCh := make(chan error)
	doneCh := make(chan bool)

	processed := uint64(0)
	go func() {
	HANDLE:
		for range workflowTicker.C {
			workflow, err := s.ds.GetNextQueueWorkflow(ctx, info.Type, info.Scope)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					errCh <- err
					return
				}
				logrus.WithError(err).Error("error getting next queue workflow")
				continue
			}
			if workflow == nil {
				continue
			}

			logrus.Debugf("received worker workflow %s", workflow.ID)

			// check if job is in job failed cache
			if kv := s.failedWorkflowCache.Get(workflow.ID); kv != nil {
				// TODO: store fail count in datastore to determine max number of failures for a single workflow
				logrus.Warnf("workflow %s is in failed cache; requeueing for another worker", workflow.ID)
				continue
			}

			// user context for updating workflow to original
			logrus.Debugf("sending workflow to subscriber %s", info.ID)
			if err := stream.Send(&api.WorkflowEvent{
				Event: &api.WorkflowEvent_Workflow{
					Workflow: workflow,
				},
			}); err != nil {
				// TODO: requeue?
				logrus.WithError(err).Errorf("error sending workflow to processor %s", info.Type)
				continue
			}

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
				case *api.SubscribeWorkflowEventsRequest_Complete:
					c := v.Complete
					logrus.Debugf("processor completed workflow %s", c.ID)
					uCtx := context.WithValue(context.Background(), flow.CtxNamespaceKey, c.Namespace)
					workflow, err := s.ds.GetWorkflow(uCtx, c.ID)
					if err != nil {
						errCh <- err
						return
					}

					workflow.Status = c.Status
					if err := s.ds.UpdateWorkflow(uCtx, workflow); err != nil {
						errCh <- err
						return
					}

					// check for max processed jobs
					processed += 1.0
					if maxWorkflows != 0 && processed >= maxWorkflows {
						logrus.Infof("worker reached max workflows (%d), exiting", maxWorkflows)
						if err := stream.Send(&api.WorkflowEvent{
							Event: &api.WorkflowEvent_Close{},
						}); err != nil {
							logrus.WithError(err).Errorf("error sending close to worker %s", c.NodeID)
						}
						doneCh <- true
						return
					}
					continue HANDLE
				default:
					errCh <- fmt.Errorf("unknown request type %s", v)
					return
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	case <-doneCh:
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

	workflowProcessors.With(prometheus.Labels{
		"type": p.Type,
	}).Inc()

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

	workflowProcessors.With(prometheus.Labels{
		"type": p.Type,
	}).Dec()
}

func (s *service) updateWorkflowOutput(ctx context.Context, o *api.WorkflowOutput) error {
	// set namespace to workflow for updating
	uCtx := context.WithValue(ctx, flow.CtxNamespaceKey, o.Namespace)
	w, err := s.ds.GetWorkflow(uCtx, o.ID)
	if err != nil {
		return err
	}

	w.UpdatedAt = time.Now()
	w.Output = o

	if err := s.ds.UpdateWorkflow(uCtx, w); err != nil {
		return err
	}

	return nil
}
