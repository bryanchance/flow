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
package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/fynca/fynca"
	api "github.com/fynca/fynca/api/services/accounts/v1"
	"github.com/fynca/fynca/datastore"
	"github.com/fynca/fynca/pkg/auth"
	"github.com/fynca/fynca/services"
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
