package render

import (
	"git.underland.io/ehazlett/finca"
	api "git.underland.io/ehazlett/finca/api/services/render/v1"
	"git.underland.io/ehazlett/finca/services"
	ptypes "github.com/gogo/protobuf/types"
	nomadapi "github.com/hashicorp/nomad/api"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	empty = &ptypes.Empty{}
)

type service struct {
	config        *finca.Config
	computeClient *nomadapi.Client
	storageClient *minio.Client
}

func New(cfg *finca.Config) (services.Service, error) {
	// storage service
	mc, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(cfg.S3AccessID, cfg.S3AccessKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, err
	}

	nomadCfg := nomadapi.DefaultConfig()
	nomadCfg.Address = cfg.NomadAddress
	if v := cfg.NomadRegion; v != "" {
		logrus.Debugf("using nomad region %s", v)
		nomadCfg.Region = v
	}
	if v := cfg.NomadNamespace; v != "" {
		logrus.Debugf("using nomad ns %s", v)
		nomadCfg.Namespace = v
	}
	nc, err := nomadapi.NewClient(nomadCfg)
	if err != nil {
		return nil, err
	}

	return &service{
		config:        cfg,
		computeClient: nc,
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
	status := s.computeClient.Status()
	leader, err := status.Leader()
	if err != nil {
		return errors.Wrap(err, "error connecting to nomad")
	}
	logrus.Debugf("connected to nomad: leader=%s", leader)

	return nil
}

func (s *service) Stop() error {
	return nil
}
