package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
	"github.com/sirupsen/logrus"
)

func (p *Postgres) GetServiceTokens(ctx context.Context) ([]*api.ServiceToken, error) {
	var serviceTokens []*api.ServiceToken
	rows, err := p.db.QueryContext(ctx, "SELECT servicetoken FROM servicetokens;")
	if err != nil {
		if err == sql.ErrNoRows {
			return serviceTokens, nil
		}
		return nil, err
	}

	for rows.Next() {
		var t *api.ServiceToken
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		serviceTokens = append(serviceTokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return serviceTokens, nil
}

func (p *Postgres) GetServiceToken(ctx context.Context, token string) (*api.ServiceToken, error) {
	var serviceToken *api.ServiceToken
	if err := p.db.QueryRowContext(ctx, "SELECT servicetoken FROM servicetokens WHERE servicetoken @> json_build_object('token', $1::text)::jsonb;", token).Scan(&serviceToken); err != nil {
		if err == sql.ErrNoRows {
			return nil, flow.ErrServiceTokenDoesNotExist
		}
		return nil, err
	}
	return serviceToken, nil
}

func (p *Postgres) CreateServiceToken(ctx context.Context, serviceToken *api.ServiceToken) error {
	// set token fields
	serviceToken.CreatedAt = time.Now()

	if _, err := p.db.Exec("INSERT INTO servicetokens (servicetoken) VALUES($1);", serviceToken); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) UpdateServiceToken(ctx context.Context, serviceToken *api.ServiceToken) error {
	logrus.Debugf("updating service token %s", serviceToken)
	if _, err := p.db.ExecContext(ctx, "UPDATE servicetokens set servicetoken = ($1) WHERE servicetoken @> json_build_object('token', $2::text)::jsonb;", serviceToken, serviceToken.Token); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) DeleteServiceToken(ctx context.Context, token string) error {
	if _, err := p.db.ExecContext(ctx, "DELETE FROM servicetokens WHERE servicetoken @> json_build_object('token', $1::text)::jsonb;", token); err != nil {
		return err
	}

	return nil
}
