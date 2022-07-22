package workflows

import (
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	ptypes "github.com/gogo/protobuf/types"
	minio "github.com/minio/minio-go/v7"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/prometheus/client_golang/prometheus"
)

func (s *service) QueueWorkflow(stream api.Workflows_QueueWorkflowServer) error {
	logrus.Debug("processing queue request")
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Unknown, "error receiving workflow: %s", err)
	}

	ctx := stream.Context()
	var (
		workflowReq  = req.GetRequest()
		buf          = bytes.Buffer{}
		workflowName = workflowReq.GetName()
		workflowSize = 0
		workflowID   = uuid.NewV4().String()
	)
	namespace, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return err
	}

	logrus.Debugf("using ns: %s", namespace)

	workflow := &api.Workflow{
		ID:         workflowID,
		Name:       workflowReq.Name,
		Type:       workflowReq.Type,
		Parameters: workflowReq.Parameters,
		Labels:     workflowReq.Labels,
		// override request namespace with context passed
		Namespace: namespace,
		CreatedAt: time.Now(),
		Priority:  workflowReq.Priority,
	}

	logrus.Debugf("processing workflow input %+v", workflowReq)
	switch v := workflowReq.Input.(type) {
	case *api.WorkflowRequest_Workflows:
		wi := []*api.WorkflowInputWorkflow{}
		for _, i := range v.Workflows.WorkflowInputs {
			// if no namespace is specified, use current
			if i.Namespace == "" {
				i.Namespace = namespace
			}
			wi = append(wi, &api.WorkflowInputWorkflow{
				Namespace: i.Namespace,
				ID:        i.ID,
			})
		}
		workflow.Input = &api.Workflow_Workflows{
			Workflows: &api.WorkflowInputWorkflows{
				WorkflowInputs: wi,
			},
		}
	case *api.WorkflowRequest_File:
		logrus.Debug("using upload content for input")
		// process user specified input data
		for {
			req, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				return status.Errorf(codes.Unknown, "error receiving workflow content: %s", err)
			}
			if err == io.EOF {
				break
			}
			c := req.GetChunkData()
			if err != nil {
				logrus.Error(err)
				return status.Errorf(codes.Unknown, "error receiving chunk data: %s", err)
			}

			workflowSize += len(c)

			if _, err := buf.Write(c); err != nil {
				logrus.Error(err)
				return status.Errorf(codes.Internal, "error saving workflow data: %s", err)
			}
		}

		// save to tmpfile to upload to s3
		tmpWorkflowFile, err := os.CreateTemp("", "flow-workflow-")
		if err != nil {
			return err
		}
		if _, err := buf.WriteTo(tmpWorkflowFile); err != nil {
			return err
		}
		tmpWorkflowFile.Close()

		defer os.Remove(tmpWorkflowFile.Name())

		// save to minio
		inputFileName := getStorageWorkflowPath(namespace, workflowID, v.File.Filename)

		logrus.Debugf("saving %s to storage", inputFileName)
		inputStorageInfo, err := s.storageClient.FPutObject(ctx, s.config.S3Bucket, inputFileName, tmpWorkflowFile.Name(), minio.PutObjectOptions{ContentType: v.File.ContentType})
		if err != nil {
			return status.Errorf(codes.Internal, "error saving workflow to storage: %s", err)
		}

		logrus.Debugf("saved workflow %s to storage service (%d bytes)", workflowName, inputStorageInfo.Size)
		workflow.Input = &api.Workflow_File{
			File: &api.WorkflowInputFile{
				Filename:    v.File.Filename,
				ContentType: v.File.ContentType,
				StoragePath: path.Dir(inputFileName),
			},
		}
	}

	// queue workflow
	logrus.Debugf("publishing workflow %s", workflow)
	if err := s.ds.CreateQueueWorkflow(ctx, workflow); err != nil {
		return err
	}

	// persist workflow to ds
	logrus.Debugf("saving workflow %s to ds", workflow.ID)
	if err := s.ds.CreateWorkflow(ctx, workflow); err != nil {
		return err
	}

	// notify client of success
	if err := stream.SendAndClose(&api.QueueWorkflowResponse{
		ID: workflowID,
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	// metrics
	workflowsProcessed.Inc()
	workflowsQueued.With(prometheus.Labels{
		"type":     workflow.Type,
		"priority": strings.ToLower(workflow.Priority.String()),
	}).Inc()

	return nil
}

func (s *service) RequeueWorkflow(ctx context.Context, r *api.RequeueWorkflowRequest) (*ptypes.Empty, error) {
	workflow, err := s.ds.GetWorkflow(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	workflow.Status = api.WorkflowStatus_PENDING
	if err := s.ds.UpdateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	if err := s.ds.CreateQueueWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	return empty, nil
}
