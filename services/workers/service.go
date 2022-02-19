package workers

import (
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/workers/v1"
	"git.underland.io/ehazlett/fynca/datastore"
	"git.underland.io/ehazlett/fynca/pkg/auth"
	"git.underland.io/ehazlett/fynca/services"
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
