package flow

import "github.com/pkg/errors"

var (
	// ErrAccountExists is returned if an account already exists
	ErrAccountExists = errors.New("an account with that username already exists")
	// ErrAccountDoesNotExist is returned when an account cannot be found
	ErrAccountDoesNotExist = errors.New("account does not exist")
	// ErrNamespaceDoesNotExist is returned when an namespace cannot be found
	ErrNamespaceDoesNotExist = errors.New("namespace does not exist")
	// ErrWorkflowDoesNotExist is returned when a workflow cannot be found
	ErrWorkflowDoesNotExist = errors.New("workflow does not exist")
	// ErrServiceTokenDoesNotExist is returned when a service token cannot be found
	ErrServiceTokenDoesNotExist = errors.New("service token does not exist")
	// ErrAPITokenDoesNotExist is returned when an api token cannot be found
	ErrAPITokenDoesNotExist = errors.New("api token does not exist")
)
