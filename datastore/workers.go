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

	api "github.com/ehazlett/flow/api/services/workers/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
)

func (d *Datastore) UpdateWorkerInfo(ctx context.Context, w *api.Worker) error {
	workerKey := getWorkerKey(w.Name)

	data, err := proto.Marshal(w)
	if err != nil {
		return err
	}
	if err := d.redisClient.Set(ctx, workerKey, data, -1).Err(); err != nil {
		return errors.Wrapf(err, "error updating worker info for %s in database", w.Name)
	}
	return nil
}

func (d *Datastore) GetWorkers(ctx context.Context) ([]*api.Worker, error) {
	keys, err := d.redisClient.Keys(ctx, getWorkerKey("*")).Result()
	if err != nil {
		return nil, err
	}

	workers := []*api.Worker{}
	for _, k := range keys {
		name := path.Base(k)
		worker, err := d.GetWorker(ctx, name)
		if err != nil {
			return nil, err
		}
		workers = append(workers, worker)
	}

	return workers, nil
}

func (d *Datastore) GetWorker(ctx context.Context, name string) (*api.Worker, error) {
	workerKey := getWorkerKey(name)
	data, err := d.redisClient.Get(ctx, workerKey).Bytes()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting worker info %s from database", name)
	}

	worker := api.Worker{}
	if err := proto.Unmarshal(data, &worker); err != nil {
		return nil, err
	}
	return &worker, nil
}

func getWorkerKey(name string) string {
	return path.Join(dbPrefix, "workers", name)
}
