package datastore

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	accountsapi "github.com/ehazlett/flow/api/services/accounts/v1"
	workflowsapi "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/ehazlett/flow/datastore/postgres"
	"github.com/ehazlett/flow/pkg/auth"
)

type Datastore interface {
	GetAccounts(ctx context.Context) ([]*accountsapi.Account, error)
	GetAccount(ctx context.Context, username string) (*accountsapi.Account, error)
	GetAccountByID(ctx context.Context, id string) (*accountsapi.Account, error)
	CreateAccount(ctx context.Context, account *accountsapi.Account) error
	UpdateAccount(ctx context.Context, account *accountsapi.Account) error
	ChangePassword(ctx context.Context, account *accountsapi.Account, password []byte) error
	DeleteAccount(ctx context.Context, username string) error

	GetAuthenticatorKey(ctx context.Context, a auth.Authenticator, key string) ([]byte, error)
	SetAuthenticatorKey(ctx context.Context, a auth.Authenticator, key string, value []byte, ttl time.Duration) error
	GetAuthenticatorKeys(ctx context.Context, a auth.Authenticator, prefix string) ([][]byte, error)

	GetNamespaces(ctx context.Context) ([]*accountsapi.Namespace, error)
	GetNamespace(ctx context.Context, id string) (*accountsapi.Namespace, error)
	CreateNamespace(ctx context.Context, namespace *accountsapi.Namespace) (string, error)
	UpdateNamespace(ctx context.Context, namespace *accountsapi.Namespace) error
	DeleteNamespace(ctx context.Context, id string) error

	GetWorkflows(ctx context.Context) ([]*workflowsapi.Workflow, error)
	GetWorkflow(ctx context.Context, id string) (*workflowsapi.Workflow, error)
	CreateWorkflow(ctx context.Context, workflow *workflowsapi.Workflow) error
	UpdateWorkflow(ctx context.Context, workflow *workflowsapi.Workflow) error
	DeleteWorkflow(ctx context.Context, id string) error

	GetServiceTokens(ctx context.Context) ([]*accountsapi.ServiceToken, error)
	GetServiceToken(ctx context.Context, token string) (*accountsapi.ServiceToken, error)
	CreateServiceToken(ctx context.Context, t *accountsapi.ServiceToken) error
	UpdateServiceToken(ctx context.Context, t *accountsapi.ServiceToken) error
	DeleteServiceToken(ctx context.Context, token string) error

	GetAPITokens(ctx context.Context) ([]*accountsapi.APIToken, error)
	GetAPIToken(ctx context.Context, token string) (*accountsapi.APIToken, error)
	CreateAPIToken(ctx context.Context, t *accountsapi.APIToken) error
	UpdateAPIToken(ctx context.Context, t *accountsapi.APIToken) error
	DeleteAPIToken(ctx context.Context, token string) error

	GetNextQueueWorkflow(ctx context.Context, queueType string, scope *workflowsapi.ProcessorScope) (*workflowsapi.Workflow, error)
	CreateQueueWorkflow(ctx context.Context, w *workflowsapi.Workflow) error
	DeleteQueueWorkflow(ctx context.Context, id string) error
}

func NewDatastore(addr string) (Datastore, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(u.Scheme) {
	case "postgres":
		return postgres.NewPostgres(addr)
	}

	return nil, fmt.Errorf("unsupported datastore type %q", u.Scheme)
}
