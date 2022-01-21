package render

import (
	"bytes"
	"context"
	"encoding/json"

	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"github.com/gogo/protobuf/jsonpb"
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

func (s *service) ControlWorker(ctx context.Context, r *api.ControlWorkerRequest) (*api.ControlWorkerResponse, error) {
	js, err := s.natsClient.JetStream()
	if err != nil {
		return nil, err
	}

	kv, err := js.KeyValue(s.config.NATSKVBucketWorkerControl)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	m := &jsonpb.Marshaler{}
	if err := m.Marshal(buf, r); err != nil {
		return nil, err
	}

	if _, err := kv.Put(r.WorkerID, buf.Bytes()); err != nil {
		return nil, err
	}

	return &api.ControlWorkerResponse{}, nil
}
