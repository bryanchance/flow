package workflows

import (
	"context"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	ptypes "github.com/gogo/protobuf/types"
	minio "github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

func (s *service) DeleteWorkflow(ctx context.Context, r *api.DeleteWorkflowRequest) (*ptypes.Empty, error) {
	workflow, err := s.ds.GetWorkflow(ctx, r.ID)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("deleting workflow %s", workflow.ID)
	if err := s.ds.DeleteQueueWorkflow(ctx, workflow.ID); err != nil {
		return nil, err
	}

	// delete from storage
	workflowStoragePath := getStoragePath(workflow.Namespace, workflow.ID)
	objCh := s.storageClient.ListObjects(ctx, s.config.S3Bucket, minio.ListObjectsOptions{
		Prefix:    workflowStoragePath,
		Recursive: true,
	})

	for o := range objCh {
		if o.Err != nil {
			return nil, o.Err
		}

		if err := s.storageClient.RemoveObject(ctx, s.config.S3Bucket, o.Key, minio.RemoveObjectOptions{}); err != nil {
			return nil, err
		}
	}

	// delete from database
	if err := s.ds.DeleteWorkflow(ctx, r.ID); err != nil {
		return nil, err
	}

	return empty, nil
}
