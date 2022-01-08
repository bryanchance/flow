package render

import (
	"context"
	"encoding/json"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	nats "github.com/nats-io/nats.go"
)

func (s *service) ListWorkers(ctx context.Context, r *api.ListWorkersRequest) (*api.ListWorkersResponse, error) {
	js, err := s.natsClient.JetStream()
	if err != nil {
		return nil, err
	}

	kv, err := js.KeyValue(s.config.NATSKVBucketNameWorkers)
	if err != nil {
		return nil, err
	}

	resp := &api.ListWorkersResponse{}

	keys, err := kv.Keys()
	if err != nil {
		if err == nats.ErrNoKeysFound {
			return resp, nil
		}
		return nil, err
	}

	workers := []*api.Worker{}

	for _, k := range keys {
		e, err := kv.Get(k)
		if err != nil {
			return nil, err
		}
		var worker *api.Worker
		if err := json.Unmarshal(e.Value(), &worker); err != nil {
			return nil, err
		}

		workers = append(workers, worker)
	}

	resp.Workers = workers
	return resp, nil
}
