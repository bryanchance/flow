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
	"io"
	"os"
	"time"

	accountsapi "github.com/ehazlett/flow/api/services/accounts/v1"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *WorkflowHandler) Run(ctx context.Context) error {
	c, err := h.getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	// check if requestor is authenticated
	if _, err := c.Authenticated(ctx, &accountsapi.AuthenticatedRequest{}); err != nil {
		if e, ok := status.FromError(err); ok {
			switch e.Code() {
			case codes.Unauthenticated:
				return errors.New("Invalid or expired authentication.  Please check your service token.")
			case codes.PermissionDenied:
				return errors.New("Access denied")
			}
		}
		return err
	}

	stream, err := c.SubscribeWorkflowEvents(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&api.SubscribeWorkflowEventsRequest{
		Request: &api.SubscribeWorkflowEventsRequest_Info{
			Info: &api.ProcessorInfo{
				ID:           h.cfg.ID,
				Type:         h.cfg.Type,
				MaxWorkflows: h.cfg.MaxWorkflows,
			},
		},
	}); err != nil {
		return err
	}

	for {
		evt, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		logrus.Debugf("workflow event: %+v", evt)
		switch v := evt.Event.(type) {
		case *api.WorkflowEvent_Workflow:
			if err := h.handleWorkflow(ctx, v.Workflow, stream); err != nil {
				logrus.WithError(err).Errorf("error handling workflow %s", v.Workflow.ID)
				continue
			}
		case *api.WorkflowEvent_Close:
			if err := v.Close.Error; err != nil {
				logrus.Error(err)
			} else {
				logrus.Debug("close event received from server")
			}
			break
		}
	}

	return nil
}

func (h *WorkflowHandler) handleWorkflow(ctx context.Context, w *api.Workflow, stream api.Workflows_SubscribeWorkflowEventsClient) error {
	// ack workflow
	if err := stream.Send(&api.SubscribeWorkflowEventsRequest{
		Request: &api.SubscribeWorkflowEventsRequest_Ack{
			Ack: &api.WorkflowAck{
				ID:        w.ID,
				Namespace: w.Namespace,
				Status:    api.WorkflowStatus_RUNNING,
			},
		},
	}); err != nil {
		return err
	}

	// process
	status := api.WorkflowStatus_COMPLETE
	// TODO: move to func to defer os.Remove this temp dir
	workflowInputDir, err := os.MkdirTemp("", "flow-workflow-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workflowInputDir)

	if err := h.unpackWorkflowInput(ctx, w, workflowInputDir); err != nil {
		return err
	}

	output := &api.WorkflowOutput{
		ID:        w.ID,
		Namespace: w.Namespace,
		StartedAt: time.Now(),
	}
	// get unpacked dir and specify configuration for Process
	processorOutput, err := h.processor.Process(ctx, &ProcessorConfig{
		Workflow: w,
		InputDir: workflowInputDir,
	})
	if err != nil {
		logrus.WithError(err).Errorf("error rendering workflow %s", w.ID)

		processorOutput = &ProcessorOutput{
			FinishedAt: time.Now(),
			Log:        err.Error(),
		}

		status = api.WorkflowStatus_ERROR
	}
	output.Info = processorOutput.Parameters
	output.FinishedAt = processorOutput.FinishedAt
	output.Duration = processorOutput.Duration
	output.Log = processorOutput.Log

	// update status
	if err := stream.Send(&api.SubscribeWorkflowEventsRequest{
		Request: &api.SubscribeWorkflowEventsRequest_Ack{
			Ack: &api.WorkflowAck{
				ID:        w.ID,
				Namespace: w.Namespace,
				Status:    status,
			},
		},
	}); err != nil {
		return errors.Wrapf(err, "error updating workflow status for %s", w.ID)
	}

	// handle artifacts
	if dir := processorOutput.OutputDir; dir != "" {
		artifacts, err := h.uploadOutputDir(ctx, w, dir)
		if err != nil {
			return errors.Wrap(err, "error uploading output artifacts")
		}
		logrus.Debugf("uploaded %d artifacts for %s", len(artifacts), w.ID)
		output.Artifacts = artifacts
		defer os.RemoveAll(dir)
	}

	// send output
	if err := stream.Send(&api.SubscribeWorkflowEventsRequest{
		Request: &api.SubscribeWorkflowEventsRequest_Output{
			Output: output,
		},
	}); err != nil {
		return errors.Wrapf(err, "error updating workflow output for %s", w.ID)
	}

	return nil
}
