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
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/datastore"
	"github.com/ehazlett/flow/pkg/auth"
	"github.com/ehazlett/flow/services"
	"github.com/ehazlett/ttlcache"
	ptypes "github.com/gogo/protobuf/types"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	nats "github.com/nats-io/nats.go"
	"github.com/olebedev/emitter"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const (
	// time to wait after a failed job to retry the workflow again
	workflowFailedCacheDelay = time.Second * 60
	// buffer size for streaming contents to clients
	bufSize = 4096
)

var (
	empty = &ptypes.Empty{}
)

type service struct {
	config              *flow.Config
	natsClient          *nats.Conn
	storageClient       *minio.Client
	ds                  *datastore.Datastore
	authenticator       auth.Authenticator
	events              *emitter.Emitter
	processors          map[string]*api.ProcessorInfo
	processorLock       *sync.Mutex
	failedWorkflowCache *ttlcache.TTLCache
}

func New(cfg *flow.Config) (services.Service, error) {
	// storage service
	mc, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(cfg.S3AccessID, cfg.S3AccessKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error setting up storage service")
	}

	// nats
	nc, err := nats.Connect(cfg.NATSURL, nats.RetryOnFailedConnect(true))
	if err != nil {
		return nil, errors.Wrap(err, "error connecting to nats")
	}

	ds, err := datastore.NewDatastore(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "error setting up datastore")
	}

	failedWorkflowCache, err := ttlcache.NewTTLCache(workflowFailedCacheDelay)
	if err != nil {
		return nil, err
	}

	return &service{
		config:              cfg,
		natsClient:          nc,
		storageClient:       mc,
		ds:                  ds,
		events:              &emitter.Emitter{},
		processors:          make(map[string]*api.ProcessorInfo),
		processorLock:       &sync.Mutex{},
		failedWorkflowCache: failedWorkflowCache,
	}, nil
}

func (s *service) Configure(a auth.Authenticator) error {
	s.authenticator = a
	return nil
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterWorkflowsServer(server, s)
	return nil
}

func (s *service) Type() services.Type {
	return services.WorkflowsService
}

func (s *service) Requires() []services.Type {
	return nil
}

func (s *service) Start() error {
	return nil
}

func (s *service) Stop() error {
	return nil
}

func getWorkflowQueueValue(w *api.Workflow) []byte {
	return []byte(path.Join(w.Namespace, w.ID))
}

// parse queue value and return as <namespace>/<id>
func parseWorkflowQueueValue(v []byte) (string, string, error) {
	parts := strings.Split(string(v), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid workflow queue value: %s", v)
	}
	return parts[0], parts[1], nil
}

func getStorageWorkflowPath(namespace, workflowID string, filename string) string {
	return path.Join(getStoragePath(namespace, workflowID), filename)
}

func getStoragePath(namespace, workflowID string) string {
	return path.Join(namespace, flow.S3WorkflowPath, workflowID)
}
