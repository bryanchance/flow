package render

import (
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/render/v1"
	"git.underland.io/ehazlett/fynca/datastore"
	"git.underland.io/ehazlett/fynca/pkg/auth"
	"git.underland.io/ehazlett/fynca/services"
	ptypes "github.com/gogo/protobuf/types"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	urgentPriorityQueueSubject = "urgent"
	lowPriorityQueueSubject    = "low"
	animationQueueSubject      = "animation"
)

var (
	// timeout for worker control messages to expire
	workerControlMessageTTL = time.Second * 60
	empty                   = &ptypes.Empty{}
)

type jobArchiveRequest struct {
	Namespace string
	ID        string
}

type service struct {
	config                  *fynca.Config
	natsClient              *nats.Conn
	storageClient           *minio.Client
	ds                      *datastore.Datastore
	serverQueueSubscription *nats.Subscription
	authenticator           auth.Authenticator
	jobArchiveCh            chan *jobArchiveRequest
	stopCh                  chan bool
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
	nc, err := nats.Connect(cfg.NATSURL)
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
		jobArchiveCh:  make(chan *jobArchiveRequest),
	}, nil
}

func (s *service) Configure(a auth.Authenticator) error {
	s.authenticator = a
	return nil
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterRenderServer(server, s)
	return nil
}

func (s *service) Type() services.Type {
	return services.RenderService
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

	// create queues
	logrus.Debugf("creating stream %s", s.config.NATSJobStreamName)
	js.AddStream(&nats.StreamConfig{
		Name: s.config.NATSJobStreamName,
		Subjects: []string{
			fynca.QueueSubjectJobPriorityNormal,
			fynca.QueueSubjectJobPriorityUrgent,
			fynca.QueueSubjectJobPriorityAnimation,
			fynca.QueueSubjectJobPriorityLow,
		},
		Retention: nats.WorkQueuePolicy,
	})
	logrus.Debugf("creating stream %s", s.config.NATSJobStatusStreamName)
	js.AddStream(&nats.StreamConfig{
		Name:      s.config.NATSJobStatusStreamName,
		Retention: nats.WorkQueuePolicy,
	})

	logrus.Debugf("job timeout: %s", s.config.JobTimeout)

	// start background listener for job updates from workers
	go s.jobStatusListener()

	// start background listener for job archive requests
	go s.jobArchiveListener()

	return nil
}

func (s *service) Stop() error {
	if sub := s.serverQueueSubscription; sub != nil {
		sub.Unsubscribe()
		sub.Drain()
	}
	return nil
}
