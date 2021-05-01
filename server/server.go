package server

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"runtime"
	"runtime/pprof"
	"sync"

	"git.underland.io/ehazlett/finca"
	"git.underland.io/ehazlett/finca/services"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	// ErrServiceRegistered is returned if an existing service is already registered for the specified type
	ErrServiceRegistered = errors.New("service is already registered for the specified type")

	empty = &ptypes.Empty{}

	// local server state db
	localDBFilename = "finca.db"
)

type Server struct {
	config           *finca.Config
	mu               *sync.Mutex
	grpcServer       *grpc.Server
	services         []services.Service
	serverCloseCh    chan bool
	serverShutdownCh chan bool
}

func NewServer(cfg *finca.Config) (*Server, error) {
	logrus.WithFields(logrus.Fields{"address": cfg.GRPCAddress}).Info("starting finca server")

	grpcOpts := []grpc.ServerOption{}
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
	grpcServer := grpc.NewServer(grpcOpts...)

	srv := &Server{
		grpcServer:       grpcServer,
		config:           cfg,
		mu:               &sync.Mutex{},
		serverCloseCh:    make(chan bool),
		serverShutdownCh: make(chan bool),
	}

	return srv, nil
}

func (s *Server) Register(svcs []func(*finca.Config) (services.Service, error)) error {
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
		go func() {
			defer wg.Done()
			logrus.Debugf("starting service %s", svc.Type())
			if err := svc.Start(); err != nil {
				serviceErrCh <- err
				return
			}
		}()
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
	tmpfile, err := ioutil.TempFile("", "finca-profile-")
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
		go func() {
			defer wg.Done()
			logrus.Debugf("stopping service %s", svc.Type())
			if err := svc.Stop(); err != nil {
				logrus.WithError(err).Errorf("error stopping service %s", svc.Type())
			}
		}()
	}

	logrus.Debug("waiting for services to shutdown")

	wg.Wait()
	return nil
}
