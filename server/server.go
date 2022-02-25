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
package server

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"

	"github.com/fynca/fynca"
	"github.com/fynca/fynca/datastore"
	"github.com/fynca/fynca/pkg/auth"
	authnone "github.com/fynca/fynca/pkg/auth/providers/none"
	authtoken "github.com/fynca/fynca/pkg/auth/providers/token"
	"github.com/fynca/fynca/pkg/middleware"
	"github.com/fynca/fynca/pkg/middleware/admin"
	"github.com/fynca/fynca/services"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

var (
	// ErrServiceRegistered is returned if an existing service is already registered for the specified type
	ErrServiceRegistered = errors.New("service is already registered for the specified type")

	empty = &ptypes.Empty{}

	// admin required routes
	adminRoutes = []string{
		"Workers/ListWorkers",
		"Workers/ControlWorker",
		// TODO: add RegisterAccount gRPC method to allow user signup
		"Accounts/CreateAccount",
		"Accounts/DeleteAccount",
	}

	publicRoutes = []string{
		"Jobs/Version",
		"Accounts/Authenticate",
	}
)

type Server struct {
	config           *fynca.Config
	mu               *sync.Mutex
	grpcServer       *grpc.Server
	services         []services.Service
	authenticator    auth.Authenticator
	serverCloseCh    chan bool
	serverShutdownCh chan bool
}

func NewServer(cfg *fynca.Config) (*Server, error) {
	logrus.WithFields(logrus.Fields{"address": cfg.GRPCAddress}).Info("starting fynca server")

	grpcOpts := []grpc.ServerOption{
		grpc.MaxMsgSize(10 * 1024 * 1024),
	}
	if cfg.TLSServerCertificate != "" && cfg.TLSServerKey != "" {
		logrus.WithFields(logrus.Fields{
			"cert": cfg.TLSServerCertificate,
			"key":  cfg.TLSServerKey,
		}).Debug("configuring TLS for GRPC")
		cert, err := tls.LoadX509KeyPair(cfg.TLSServerCertificate, cfg.TLSServerKey)
		if err != nil {
			return nil, err
		}
		creds := credentials.NewTLS(&tls.Config{
			Certificates:       []tls.Certificate{cert},
			ClientAuth:         tls.RequestClientCert,
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
		})
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
	}

	defaultNoneAuthenticator := &authnone.NoneAuthenticator{}

	// setup default authenticator
	if cfg.Authenticator == nil {
		cfg.Authenticator = &fynca.AuthenticatorConfig{Name: "none"}
	}

	// interceptors
	unaryServerInterceptors := []grpc.UnaryServerInterceptor{}
	streamServerInterceptors := []grpc.StreamServerInterceptor{}

	ds, err := datastore.NewDatastore(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "error setting up datastore")
	}

	var authenticator auth.Authenticator
	// middleware
	grpcMiddleware := []middleware.Middleware{}

	switch strings.ToLower(cfg.Authenticator.Name) {
	case "none":
		authenticator = defaultNoneAuthenticator
	case "token":
		authenticator = authtoken.NewTokenAuthenticator(ds, publicRoutes)
		// admin required for token auth
		grpcMiddleware = append(grpcMiddleware, admin.NewAdminRequired(authenticator, adminRoutes, publicRoutes))
	default:
		return nil, fmt.Errorf("unknown authenticator %s", cfg.Authenticator.Name)
	}
	unaryServerInterceptors = append(unaryServerInterceptors, authenticator.UnaryServerInterceptor)
	streamServerInterceptors = append(streamServerInterceptors, authenticator.StreamServerInterceptor)

	logrus.Debugf("loaded authenticator %s", authenticator.Name())

	// telemetry
	unaryServerInterceptors = append(unaryServerInterceptors, otelgrpc.UnaryServerInterceptor())
	streamServerInterceptors = append(streamServerInterceptors, otelgrpc.StreamServerInterceptor())

	for _, m := range grpcMiddleware {
		unaryServerInterceptors = append(unaryServerInterceptors, m.UnaryServerInterceptor)
		streamServerInterceptors = append(streamServerInterceptors, m.StreamServerInterceptor)
	}

	grpcOpts = append(grpcOpts,
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryServerInterceptors...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamServerInterceptors...)),
	)
	grpcServer := grpc.NewServer(grpcOpts...)

	srv := &Server{
		grpcServer:       grpcServer,
		config:           cfg,
		authenticator:    authenticator,
		mu:               &sync.Mutex{},
		serverCloseCh:    make(chan bool),
		serverShutdownCh: make(chan bool),
	}

	return srv, nil
}

func (s *Server) Register(svcs []func(*fynca.Config) (services.Service, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// register services from caller
	registered := map[services.Type]struct{}{}
	for _, svc := range svcs {
		i, err := svc(s.config)
		if err != nil {
			return err
		}
		if err := i.Register(s.grpcServer); err != nil {
			return err
		}

		// configure
		if err := i.Configure(s.authenticator); err != nil {
			return err
		}

		// check for existing service
		if _, exists := registered[i.Type()]; exists {
			return errors.Wrap(ErrServiceRegistered, string(i.Type()))
		}
		logrus.WithFields(logrus.Fields{
			"type": i.Type(),
		}).Info("registered service")
		registered[i.Type()] = struct{}{}
		s.services = append(s.services, i)
	}

	return nil
}

func (s *Server) Run() error {
	l, err := net.Listen("tcp", s.config.GRPCAddress)
	if err != nil {
		return err
	}

	doneCh := make(chan bool)
	serviceErrCh := make(chan error)
	wg := &sync.WaitGroup{}
	for _, svc := range s.services {
		wg.Add(1)
		go func(sv services.Service) {
			defer wg.Done()
			logrus.Debugf("starting service %s", sv.Type())
			if err := sv.Start(); err != nil {
				serviceErrCh <- err
				return
			}
		}(svc)
	}

	go func() {
		logrus.Debug("waiting for services start")
		wg.Wait()
		doneCh <- true
	}()

	select {
	case <-doneCh:
	case err := <-serviceErrCh:
		return err
	}

	errCh := make(chan error)
	logrus.WithField("addr", s.config.GRPCAddress).Debug("starting grpc server")
	go s.grpcServer.Serve(l)

	go func() {
		for {
			err := <-errCh
			logrus.Error(err)
		}
	}()

	return nil
}

func (s *Server) GenerateProfile() (string, error) {
	tmpfile, err := ioutil.TempFile("", "fynca-profile-")
	if err != nil {
		return "", err
	}
	runtime.GC()
	if err := pprof.WriteHeapProfile(tmpfile); err != nil {
		return "", err
	}
	tmpfile.Close()
	return tmpfile.Name(), nil
}

func (s *Server) Stop() error {
	logrus.Debug("stopping server")

	// stop services
	wg := &sync.WaitGroup{}
	for _, svc := range s.services {
		wg.Add(1)
		go func(sv services.Service) {
			defer wg.Done()
			logrus.Debugf("stopping service %s", sv.Type())
			if err := sv.Stop(); err != nil {
				logrus.WithError(err).Errorf("error stopping service %s", svc.Type())
			}
		}(svc)
	}

	logrus.Debug("waiting for services to shutdown")

	wg.Wait()
	return nil
}
