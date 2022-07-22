package workflows

import (
	"bytes"
	"io"
	"os"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
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
		workflowID          = uploadArtifact.GetID()
		artifactFilename    = uploadArtifact.GetFilename()
		artifactNamespace   = uploadArtifact.GetNamespace()
		artifactContentType = uploadArtifact.GetContentType()
		artifactSize        = 0
	)
	logrus.Debugf("processing workflow upload  %+v", uploadArtifact)

	// process user specified input data
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "error receiving workflow upload: %s", err)
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
	tmpUploadFile, err := os.CreateTemp("", "flow-workflow-upload-")
	if err != nil {
		return err
	}
	if _, err := buf.WriteTo(tmpUploadFile); err != nil {
		return err
	}
	tmpUploadFile.Close()

	defer os.Remove(tmpUploadFile.Name())

	// save to minio
	uploadKey := getStorageWorkflowPath(artifactNamespace, workflowID, artifactFilename)

	logrus.Debugf("saving %s to storage", uploadKey)
	uploadInfo, err := s.storageClient.FPutObject(ctx, s.config.S3Bucket, uploadKey, tmpUploadFile.Name(), minio.PutObjectOptions{ContentType: artifactContentType})
	if err != nil {
		return status.Errorf(codes.Internal, "error saving workflow upload to storage: %s", err)
	}

	logrus.Debugf("saved workflow upload %s to storage service (%d bytes)", uploadKey, uploadInfo.Size)

	// notify client of success
	if err := stream.SendAndClose(&api.UploadWorkflowArtifactResponse{
		StoragePath: uploadKey,
	}); err != nil {
		return status.Errorf(codes.Unknown, "error sending response to client: %s", err)
	}

	return nil
}
