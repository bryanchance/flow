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
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workflows/v1"
	"github.com/gogo/protobuf/proto"
	minio "github.com/minio/minio-go/v7"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *service) QueueWorkflow(stream api.Workflows_QueueWorkflowServer) error {
	js, err := s.natsClient.JetStream()
	if err != nil {
		return err
	}

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
	namespace, err := fynca.GetNamespaceFromContext(ctx)
	if err != nil {
		return err
	}

	logrus.Debugf("using ns: %s", namespace)

	workflow := &api.Workflow{
		ID:         workflowID,
		Name:       workflowReq.Name,
		Type:       workflowReq.Type,
		Parameters: workflowReq.Parameters,
		// override request namespace with context passed
		Namespace: namespace,
		CreatedAt: time.Now(),
		Priority:  workflowReq.Priority,
	}

	logrus.Debugf("processing workflow input %+v", workflowReq)
	switch v := workflowReq.Input.(type) {
	case *api.WorkflowRequest_Workflows:
		// TODO: get input workflow id storage path
		logrus.Debugf("using workflows %s for input", v.Workflows.IDs)
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
		tmpJobFile, err := os.CreateTemp("", "fynca-workflow-")
		if err != nil {
			return err
		}
		if _, err := buf.WriteTo(tmpJobFile); err != nil {
			return err
		}
		tmpJobFile.Close()

		defer os.Remove(tmpJobFile.Name())

		// save to minio
		inputFileName := getStorageWorkflowPath(namespace, workflowID, v.File.Filename)

		logrus.Debugf("saving %s to storage", inputFileName)
		inputStorageInfo, err := s.storageClient.FPutObject(ctx, s.config.S3Bucket, inputFileName, tmpJobFile.Name(), minio.PutObjectOptions{ContentType: v.File.ContentType})
		if err != nil {
			return status.Errorf(codes.Internal, "error saving workflow to storage: %s", err)
		}

		logrus.Debugf("saved workflow %s to storage service (%d bytes)", workflowName, inputStorageInfo.Size)
		workflow.InputPath = path.Dir(inputFileName)
	}

	// queue workflow
	// TODO: get workflow priority
	workflowSubject := getWorkflowSubject(workflow)
	logrus.Debugf("using nats subject: %s", workflowSubject)
	data, err := proto.Marshal(workflow)
	if err != nil {
		return err
	}

	logrus.Debugf("publishing workflow %s", workflow)
	ack, err := js.Publish(workflowSubject, data)
	if err != nil {
		return err
	}
	logrus.Debugf("workflow sequence id: %d", ack.Sequence)
	workflow.SequenceID = ack.Sequence

	logrus.Debugf("workflow: %+v", workflow)

	// persist workflow to ds
	logrus.Debugf("saving workflow %s to ds", workflow.ID)
	if err := s.ds.UpdateWorkflow(ctx, workflow); err != nil {
		return err
	}

	// notify client of success
	if err := stream.SendAndClose(&api.QueueWorkflowResponse{
		ID: workflowID,
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	return nil
}

func getWorkflowSubject(w *api.Workflow) string {
	priority := strings.ToLower(w.Priority.String())
	// TODO: priority
	return fmt.Sprintf("workflows.%s.%s", priority, w.Type)
}

func getStorageWorkflowPath(namespace, workflowID string, filename string) string {
	return path.Join(getStoragePath(namespace, workflowID), filename)
}

func getStoragePath(namespace, workflowID string) string {
	return path.Join(namespace, fynca.S3WorkflowPath, workflowID)
}
