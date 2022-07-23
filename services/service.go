package services

import (
	"github.com/ehazlett/flow/pkg/auth"
	"google.golang.org/grpc"
)

type Type string

const (
	AccountsService  Type = "flow.services.accounts.v1"
	InfoService      Type = "flow.services.info.v1"
	WorkersService   Type = "flow.services.workers.v1"
	WorkflowsService Type = "flow.services.workflows.v1"
)

// Service is the interface that all services must implement
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
