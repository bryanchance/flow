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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	minio "github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (s *service) GetWorkflowOutputArtifact(r *api.GetWorkflowOutputArtifactRequest, stream api.Workflows_GetWorkflowOutputArtifactServer) error {
	ctx := stream.Context()

	workflow, err := s.ds.GetWorkflow(ctx, r.ID)
	if err != nil {
		return err
	}

	logrus.Debugf("getting artifact %s from workflow %s", r.Name, workflow.ID)

	var artifact *api.WorkflowOutputArtifact

	for _, a := range workflow.Output.Artifacts {
		if strings.ToLower(a.Name) == strings.ToLower(r.Name) {
			artifact = a
			break
		}
	}

	if artifact == nil {
		return fmt.Errorf("unable to find output artifact in workflow %s with name %s", r.ID, r.Name)
	}

	tmpOutputFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("flow-output-artifact-%s", getHash(artifact.StoragePath, time.Now().String())))
	defer os.Remove(tmpOutputFilePath)

	if err := s.storageClient.FGetObject(ctx, s.config.S3Bucket, artifact.StoragePath, tmpOutputFilePath, minio.GetObjectOptions{}); err != nil {
		return err
	}

	f, err := os.Open(tmpOutputFilePath)
	if err != nil {
		return err
	}
	rdr := bufio.NewReader(f)
	buf := make([]byte, bufSize)

	if err := stream.Send(&api.WorkflowOutputArtifactContents{
		Data: &api.WorkflowOutputArtifactContents_Artifact{
			Artifact: artifact,
		},
	}); err != nil {
		return err
	}

	chunk := 0
	for {
		n, err := rdr.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "error reading file chunk")
		}

		resp := &api.WorkflowOutputArtifactContents{
			Data: &api.WorkflowOutputArtifactContents_ChunkData{
				ChunkData: buf[:n],
			},
		}

		if err := stream.Send(resp); err != nil {
			return errors.Wrap(err, "error sending file chunk")
		}

		chunk += 1
	}

	return nil
}
