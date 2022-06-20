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
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workers/v1"
	"github.com/ehazlett/flow/datastore"
	"github.com/ehazlett/flow/pkg/auth"
	"github.com/ehazlett/flow/services"
	ptypes "github.com/gogo/protobuf/types"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

var (
	// timeout for worker control messages to expire
	workerControlMessageTTL = time.Second * 60
	empty                   = &ptypes.Empty{}
)

type service struct {
	config        *fynca.Config
	natsClient    *nats.Conn
	storageClient *minio.Client
	ds            *datastore.Datastore
	authenticator auth.Authenticator
}

func New(cfg *fynca.Config) (services.Service, error) {
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

	return &service{
		config:        cfg,
		natsClient:    nc,
		storageClient: mc,
		ds:            ds,
	}, nil
}

func (s *service) Configure(a auth.Authenticator) error {
	s.authenticator = a
	return nil
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterWorkersServer(server, s)
	return nil
}

func (s *service) Type() services.Type {
	return services.WorkersService
}

func (s *service) Requires() []services.Type {
	return nil
}

func (s *service) Start() error {
	js, err := s.natsClient.JetStream()
	if err != nil {
		return errors.Wrap(err, "error getting nats jetstream context")
	}

	// kv for worker control
	if _, err := js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: s.config.NATSKVBucketWorkerControl,
		TTL:    workerControlMessageTTL,
	}); err != nil {
		return errors.Wrapf(err, "error creating kv bucket %s", s.config.NATSKVBucketWorkerControl)
	}

	return nil
}

func (s *service) Stop() error {
	return nil
}
