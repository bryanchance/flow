package render

import (
	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"git.underland.io/ehazlett/finca/services"
	ptypes "github.com/gogo/protobuf/types"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	nats "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	empty = &ptypes.Empty{}
)

type service struct {
	config        *finca.Config
	natsClient    *nats.Conn
	storageClient *minio.Client
}

func New(cfg *finca.Config) (services.Service, error) {
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

	js, err := nc.JetStream()
	if err != nil {
		return nil, errors.Wrap(err, "error getting nats jetstream context")
	}

	js.AddStream(&nats.StreamConfig{
		Name:     cfg.NATSSubject,
		Subjects: []string{cfg.NATSSubject + ".*"},
	})

	logrus.Debugf("job timeout: %s", cfg.JobTimeout)

	return &service{
		config:        cfg,
		natsClient:    nc,
		storageClient: mc,
	}, nil
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
	return nil
}

func (s *service) Stop() error {
	return nil
}
