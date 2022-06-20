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
	"io"
	"os"
	"path/filepath"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (h *WorkflowHandler) uploadOutputDir(ctx context.Context, w *api.Workflow, outputDir string) ([]*api.WorkflowOutputArtifact, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}

	artifacts := []*api.WorkflowOutputArtifact{}

	logrus.Debugf("uploading output from %s", outputDir)
	files, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return nil, err
	}
	logrus.Debugf("using ns: %s", w.Namespace)

	for _, f := range files {
		logrus.Debugf("uploading output file %s", f)
		stream, err := c.UploadWorkflowArtifact(ctx)
		if err != nil {
			return nil, err
		}

		rf, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		defer rf.Close()

		fs, err := rf.Stat()
		if err != nil {
			return nil, err
		}
		// TODO: build full dir path ?
		if fs.IsDir() {
			continue
		}

		contentType, err := fynca.GetContentType(rf)
		if err != nil {
			return nil, err
		}

		// send artifact info
		if err := stream.Send(&api.UploadWorkflowArtifactRequest{
			Data: &api.UploadWorkflowArtifactRequest_Artifact{
				Artifact: &api.WorkflowOutputArtifactUpload{
					ID:          w.ID,
					Namespace:   w.Namespace,
					Filename:    filepath.Base(f),
					ContentType: contentType,
				},
			},
		}); err != nil {
			return nil, err
		}

		rdr := bufio.NewReader(rf)
		buf := make([]byte, bufSize)

		chunk := 0
		for {
			n, err := rdr.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, errors.Wrap(err, "error reading file chunk")
			}

			req := &api.UploadWorkflowArtifactRequest{
				Data: &api.UploadWorkflowArtifactRequest_ChunkData{
					ChunkData: buf[:n],
				},
			}

			if err := stream.Send(req); err != nil {
				if err == io.EOF {
					break
				}
				return nil, errors.Wrap(err, "error sending file chunk")
			}

			chunk += 1
		}

		resp, err := stream.CloseAndRecv()
		if err != nil {
			return nil, errors.Wrap(err, "error receiving upload response from server")
		}

		logrus.Debugf("uploaded %d bytes", chunk*bufSize)

		artifacts = append(artifacts, &api.WorkflowOutputArtifact{
			Name:        filepath.Base(f),
			ContentType: contentType,
			StoragePath: resp.StoragePath,
		})
	}

	return artifacts, nil
}
