package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
	uuid "github.com/satori/go.uuid"
)

func (p *Postgres) GetNamespaces(ctx context.Context) ([]*api.Namespace, error) {
	var namespaces []*api.Namespace
	rows, err := p.db.QueryContext(ctx, "SELECT namespace FROM namespaces;")
	if err != nil {
		if err == sql.ErrNoRows {
			return namespaces, nil
		}
		return nil, err
	}

	for rows.Next() {
		var n *api.Namespace
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		namespaces = append(namespaces, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return namespaces, nil
}

func (p *Postgres) GetNamespace(ctx context.Context, id string) (*api.Namespace, error) {
	var namespace *api.Namespace
	if err := p.db.QueryRowContext(ctx, "SELECT namespace FROM namespaces WHERE namespace @> json_build_object('id', $1::text)::jsonb;", id).Scan(&namespace); err != nil {
		if err == sql.ErrNoRows {
			return nil, flow.ErrAccountDoesNotExist
		}
		return nil, err
	}
	return namespace, nil
}

func (p *Postgres) CreateNamespace(ctx context.Context, namespace *api.Namespace) (string, error) {
	id := uuid.NewV4().String()

	// validate
	if namespace.Name == "" {
		return "", fmt.Errorf("name cannot be blank")
	}

	// override id field to ensure unique
	namespace.ID = id
	// set namespace fields
	namespace.CreatedAt = time.Now()

	if _, err := p.db.Exec("INSERT INTO namespaces (namespace) VALUES($1);", namespace); err != nil {
		return "", err
	}

	return id, nil
}

func (p *Postgres) UpdateNamespace(ctx context.Context, namespace *api.Namespace) error {
	if _, err := p.db.ExecContext(ctx, "UPDATE namespaces set namespace = ($1) WHERE namespace @> json_build_object('id', $2::text)::jsonb;", namespace, namespace.ID); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) DeleteNamespace(ctx context.Context, id string) error {
	if _, err := p.db.ExecContext(ctx, "DELETE FROM namespaces WHERE namespace @> json_build_object('id', $1::text)::jsonb;", id); err != nil {
		return err
	}

	return nil
}
