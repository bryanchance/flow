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
	"io"
	"os"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workflows/v1"
	minio "github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *service) UploadWorkflowArtifact(stream api.Workflows_UploadWorkflowArtifactServer) error {
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Unknown, "error receiving workflow artifact: %s", err)
	}

	ctx := stream.Context()
	var (
		uploadArtifact      = req.GetArtifact()
		buf                 = bytes.Buffer{}
		workflowID          = uploadArtifact.GetWorkflowID()
		artifactFilename    = uploadArtifact.GetFilename()
		artifactContentType = uploadArtifact.GetContentType()
		artifactSize        = 0
	)
	namespace, err := fynca.GetNamespaceFromContext(ctx)
	if err != nil {
		return err
	}

	logrus.Debugf("upload using ns: %s", namespace)

	logrus.Debugf("processing workflow upload  %+v", uploadArtifact)
	// process user specified input data
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return status.Errorf(codes.Unknown, "error receiving workflow upload: %s", err)
		}
		if err == io.EOF {
			break
		}
		c := req.GetChunkData()
		if err != nil {
			logrus.Error(err)
			return status.Errorf(codes.Unknown, "error receiving chunk data: %s", err)
		}

		artifactSize += len(c)

		if _, err := buf.Write(c); err != nil {
			logrus.Error(err)
			return status.Errorf(codes.Internal, "error saving workflow data: %s", err)
		}
	}

	// save to tmpfile to upload to s3
	tmpUploadFile, err := os.CreateTemp("", "fynca-workflow-upload-")
	if err != nil {
		return err
	}
	if _, err := buf.WriteTo(tmpUploadFile); err != nil {
		return err
	}
	tmpUploadFile.Close()

	defer os.Remove(tmpUploadFile.Name())

	// save to minio
	uploadKey := getStorageWorkflowPath(namespace, workflowID, artifactFilename)

	logrus.Debugf("saving %s to storage", uploadKey)
	uploadInfo, err := s.storageClient.FPutObject(ctx, s.config.S3Bucket, artifactFilename, tmpUploadFile.Name(), minio.PutObjectOptions{ContentType: artifactContentType})
	if err != nil {
		return status.Errorf(codes.Internal, "error saving workflow upload to storage: %s", err)
	}

	logrus.Debugf("saved workflow upload %s to storage service (%d bytes)", artifactFilename, uploadInfo.Size)

	// notify client of success
	if err := stream.SendAndClose(&api.UploadWorkflowArtifactResponse{
		StoragePath: uploadKey,
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	return nil
}
