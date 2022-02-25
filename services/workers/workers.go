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
package workers

import (
	"context"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/workers/v1"
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

	// inject requestor
	username := ctx.Value(fynca.CtxUsernameKey).(string)
	r.Requestor = username

	data, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}

	if _, err := kv.Put(r.WorkerID, data); err != nil {
		return nil, err
	}

	return &api.ControlWorkerResponse{}, nil
}
