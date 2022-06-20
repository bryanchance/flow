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
	"time"

	"github.com/ehazlett/ttlcache"
	"github.com/ehazlett/flow"
	"github.com/ehazlett/flow/client"
)

const (
	// time to wait after a failed job to retry the workflow again
	workflowFailedCacheDelay = time.Second * 60

	bufSize = 4096
)

type WorkflowHandler struct {
	cfg                 *Config
	failedWorkflowCache *ttlcache.TTLCache
	stopCh              chan bool
	processor           Processor
}

func NewWorkflowHandler(cfg *Config, p Processor) (*WorkflowHandler, error) {
	// TODO: validate cfg
	failedWorkflowCache, err := ttlcache.NewTTLCache(workflowFailedCacheDelay)
	if err != nil {
		return nil, err
	}

	return &WorkflowHandler{
		cfg:                 cfg,
		failedWorkflowCache: failedWorkflowCache,
		stopCh:              make(chan bool, 1),
		processor:           p,
	}, nil
}

func (h *WorkflowHandler) getClient() (*client.Client, error) {
	cfg := &fynca.Config{
		GRPCAddress:           h.cfg.Address,
		TLSClientCertificate:  h.cfg.TLSCertificate,
		TLSClientKey:          h.cfg.TLSKey,
		TLSInsecureSkipVerify: h.cfg.TLSInsecureSkipVerify,
	}

	return client.NewClient(cfg, client.WithServiceToken(h.cfg.ServiceToken))
}
