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

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/pkg/queue"
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

	// delete from queue
	workflowQueue, err := queue.NewQueue(s.config.QueueAddress)
	if err != nil {
		return nil, err
	}

	priority, err := getWorkflowQueuePriority(workflow.Priority)
	if err != nil {
		return nil, err
	}

	v := getWorkflowQueueValue(workflow)
	if err := workflowQueue.Delete(ctx, workflow.Namespace, workflow.Type, v, priority); err != nil {
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
