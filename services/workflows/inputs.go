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
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (s *service) ListWorkflowInputs(ctx context.Context, r *api.ListWorkflowInputsRequest) (*api.ListWorkflowInputsResponse, error) {
	uCtx := context.WithValue(ctx, flow.CtxNamespaceKey, r.Namespace)
	workflow, err := s.ds.GetWorkflow(uCtx, r.ID)
	if err != nil {
		return nil, err
	}
	prefixes := []string{}
	workflowInputFiles := []*api.WorkflowInputFile{}

	switch f := workflow.Input.(type) {
	case *api.Workflow_Workflows:
		for _, x := range f.Workflows.WorkflowInputs {
			iwf, err := s.ds.GetWorkflow(uCtx, x.ID)
			if err != nil {
				return nil, err
			}
			for _, o := range iwf.Output.Artifacts {
				prefixes = append(prefixes, o.StoragePath)
			}
		}
	case *api.Workflow_File:
		prefixes = append(prefixes, f.File.StoragePath)
	}

	logrus.Debugf("workflow storage prefixes: %+v", prefixes)
	for _, prefix := range prefixes {
		oCh := s.storageClient.ListObjects(ctx, s.config.S3Bucket, minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		})
		for o := range oCh {
			workflowInputFiles = append(workflowInputFiles, &api.WorkflowInputFile{
				Filename:    path.Base(o.Key),
				ContentType: o.ContentType,
				StoragePath: o.Key,
			})
		}
	}

	return &api.ListWorkflowInputsResponse{
		Files: workflowInputFiles,
	}, nil
}

func (s *service) GetWorkflowInputFile(r *api.GetWorkflowInputFileRequest, stream api.Workflows_GetWorkflowInputFileServer) error {
	ctx := stream.Context()

	logrus.Debugf("getting workflow input from from storage: %s", r.StoragePath)
	tmpInputFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("flow-input-%s", getHash(r.StoragePath, time.Now().String())))
	defer os.Remove(tmpInputFilePath)

	if err := s.storageClient.FGetObject(ctx, s.config.S3Bucket, r.StoragePath, tmpInputFilePath, minio.GetObjectOptions{}); err != nil {
		return err
	}

	// stream to client
	logrus.Debugf("streaming workflow input file %s to client", r.StoragePath)
	f, err := os.Open(tmpInputFilePath)
	if err != nil {
		return err
	}
	rdr := bufio.NewReader(f)
	buf := make([]byte, bufSize)

	chunk := 0
	for {
		n, err := rdr.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "error reading file chunk")
		}

		req := &api.WorkflowInputFileContents{
			ChunkData: buf[:n],
		}

		if err := stream.Send(req); err != nil {
			return errors.Wrap(err, "error sending file chunk")
		}

		chunk += 1
	}

	logrus.Debugf("sent %s to client (%d bytes)", r.StoragePath, chunk*bufSize)

	return nil
}

func getHash(v ...string) string {
	h := sha1.New()
	for _, x := range v {
		_, _ = io.WriteString(h, x)
	}
	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:])
}
