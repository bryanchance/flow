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
package datastore

import (
	"context"
	"path"
	"sort"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ByWorkflowCreatedAt []*api.Workflow

func (s ByWorkflowCreatedAt) Len() int           { return len(s) }
func (s ByWorkflowCreatedAt) Less(i, j int) bool { return s[i].CreatedAt.After(s[j].CreatedAt) }
func (s ByWorkflowCreatedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (d *Datastore) GetWorkflows(ctx context.Context) ([]*api.Workflow, error) {
	namespace, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := d.redisClient.Keys(ctx, getWorkflowKey(namespace, "*")).Result()
	if err != nil {
		return nil, err
	}

	workflows := []*api.Workflow{}
	for _, k := range keys {
		id := path.Base(k)
		workflow, err := d.GetWorkflow(ctx, id)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, workflow)
	}
	sort.Sort(ByWorkflowCreatedAt(workflows))

	return workflows, nil
}

func (d *Datastore) GetWorkflow(ctx context.Context, id string) (*api.Workflow, error) {
	namespace, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return nil, err
	}
	workflowKey := getWorkflowKey(namespace, id)
	data, err := d.redisClient.Get(ctx, workflowKey).Bytes()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting workflow %s from database", id)
	}

	workflow := api.Workflow{}
	if err := proto.Unmarshal(data, &workflow); err != nil {
		return nil, err
	}
	return &workflow, nil
}

func (d *Datastore) UpdateWorkflow(ctx context.Context, workflow *api.Workflow) error {
	namespace, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return err
	}
	workflowKey := getWorkflowKey(namespace, workflow.ID)

	// set UpdatedAt
	workflow.UpdatedAt = time.Now()

	logrus.Debugf("datastore: updating workflow: %+v", workflow)

	data, err := proto.Marshal(workflow)
	if err != nil {
		return err
	}
	if err := d.redisClient.Set(ctx, workflowKey, data, 0).Err(); err != nil {
		return errors.Wrapf(err, "error updating workflow for %s in database", workflow.ID)
	}
	return nil
}

func (d *Datastore) DeleteWorkflow(ctx context.Context, id string) error {
	// ensure exists
	if _, err := d.GetWorkflow(ctx, id); err != nil {
		return err
	}
	namespace, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return err
	}
	workflowKey := getWorkflowKey(namespace, id)

	if err := d.redisClient.Del(ctx, workflowKey).Err(); err != nil {
		return err
	}

	return nil
}

func getWorkflowKey(namespace, id string) string {
	return path.Join(dbPrefix, namespace, "workflows", id)
}
