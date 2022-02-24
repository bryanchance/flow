package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"git.underland.io/ehazlett/fynca"
	api "git.underland.io/ehazlett/fynca/api/services/accounts/v1"
	"git.underland.io/ehazlett/fynca/datastore"
	"git.underland.io/ehazlett/fynca/pkg/auth"
	"git.underland.io/ehazlett/fynca/services"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	empty = &ptypes.Empty{}
)

type service struct {
	config        *fynca.Config
	authenticator auth.Authenticator
	ds            *datastore.Datastore
}

func New(cfg *fynca.Config) (services.Service, error) {
	ds, err := datastore.NewDatastore(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "error setting up datastore")
	}

	return &service{
		config: cfg,
		ds:     ds,
	}, nil
}

func (s *service) Configure(a auth.Authenticator) error {
	s.authenticator = a
	return nil
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterAccountsServer(server, s)
	return nil
}

func (s *service) Type() services.Type {
	return services.AccountsService
}

func (s *service) Requires() []services.Type {
	return nil
}

func (s *service) Start() error {
	// check for admin account and create if missing
	ctx := context.Background()
	if _, err := s.ds.GetAccount(ctx, "admin"); err != nil {
		if err != datastore.ErrAccountDoesNotExist {
			return err
		}
		// create
		tmpPassword := s.config.InitialAdminPassword
		if tmpPassword == "" {
			hash := sha256.Sum256([]byte(fmt.Sprintf("%s", time.Now())))
			tmpPassword = hex.EncodeToString(hash[:10])
		}
		logrus.Debugf("passwd: %s", tmpPassword)

		adminAcct := &api.Account{
			Username:  "admin",
			FirstName: "Fynca",
			LastName:  "Admin",
			Admin:     true,
			Password:  tmpPassword,
		}
		if err := s.ds.CreateAccount(ctx, adminAcct); err != nil {
			return err
		}

		logrus.Infof("created admin account: username=%s password=%s", adminAcct.Username, tmpPassword)
	}
	return nil
}

func (s *service) Stop() error {
	return nil
}
