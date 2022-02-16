package render

import (
	"context"

	api "git.underland.io/ehazlett/fynca/api/services/jobs/v1"
	"github.com/gogo/protobuf/proto"
)

func (s *service) ListWorkers(ctx context.Context, r *api.ListWorkersRequest) (*api.ListWorkersResponse, error) {
	resp := &api.ListWorkersResponse{}

	workers, err := s.ds.GetWorkers(ctx)
	if err != nil {
		return nil, err
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

	data, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}

	if _, err := kv.Put(r.WorkerID, data); err != nil {
		return nil, err
	}

	return &api.ControlWorkerResponse{}, nil
}
