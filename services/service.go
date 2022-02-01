package services

import (
	"git.underland.io/ehazlett/fynca/pkg/auth"
	"google.golang.org/grpc"
)

type Type string

const (
	RenderService   Type = "fynca.services.render.v1"
	AccountsService Type = "fynca.services.accounts.v1"
)

// Service is the interface that all stellar services must implement
type Service interface {
	// Type returns the type that the service provides
	Type() Type
	// Register registers the service with the GRPC server
	Register(*grpc.Server) error
	// Configure configures the service
	Configure(auth.Authenticator) error
	// Start provides a mechanism to start service specific actions
	Start() error
	// Stop provides a mechanism to stop the service
	Stop() error
}
