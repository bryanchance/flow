package info

import (
	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/info/v1"
	"github.com/ehazlett/flow/pkg/auth"
	"github.com/ehazlett/flow/services"
	ptypes "github.com/gogo/protobuf/types"
	"google.golang.org/grpc"
)

var (
	empty = &ptypes.Empty{}
)

type service struct {
	config        *flow.Config
	authenticator auth.Authenticator
}

func New(cfg *flow.Config) (services.Service, error) {
	return &service{
		config: cfg,
	}, nil
}

func (s *service) Configure(a auth.Authenticator) error {
	s.authenticator = a
	return nil
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterInfoServer(server, s)
	return nil
}

func (s *service) Type() services.Type {
	return services.InfoService
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
