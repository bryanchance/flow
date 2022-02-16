package datastore

import (
	"context"
	"path"
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/jobs/v1"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
)

func (d *Datastore) UpdateWorkerInfo(ctx context.Context, w *api.Worker) error {
	workerKey := getWorkerKey(w.Name)

	data, err := proto.Marshal(w)
	if err != nil {
		return err
	}
	keyTTL := fynca.WorkerTTL + time.Second*1
	if err := d.redisClient.Set(ctx, workerKey, data, keyTTL).Err(); err != nil {
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
