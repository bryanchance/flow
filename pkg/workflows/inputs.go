package workflows

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *WorkflowHandler) unpackWorkflowInput(ctx context.Context, w *api.Workflow, dest string) error {
	c, err := h.getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	resp, err := c.ListWorkflowInputs(ctx, &api.ListWorkflowInputsRequest{
		ID:        w.ID,
		Namespace: w.Namespace,
	})
	if err != nil {
		return err
	}

	for _, o := range resp.Files {
		localPath := filepath.Join(dest, o.Filename)
		if err := h.getWorkflowInputFile(ctx, o.StoragePath, localPath); err != nil {
			return err
		}
	}

	return nil
}

func (h *WorkflowHandler) getWorkflowInputFile(ctx context.Context, src, dest string) error {
	c, err := h.getClient()
	if err != nil {
		return err
	}
	defer c.Close()

	buf := bytes.Buffer{}
	fileSize := 0

	stream, err := c.GetWorkflowInputFile(ctx, &api.GetWorkflowInputFileRequest{
		StoragePath: src,
	})
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

		fileSize += len(c)

		if _, err := buf.Write(c); err != nil {
			logrus.Error(err)
			return status.Errorf(codes.Internal, "error saving workflow data: %s", err)
		}
	}

	// save to tmpfile to upload to s3
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	outputFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := buf.WriteTo(outputFile); err != nil {
		return err
	}
	outputFile.Close()

	logrus.Debugf("saved workflow input file %s to %s (%d bytes)", src, dest, fileSize)

	return nil
}
