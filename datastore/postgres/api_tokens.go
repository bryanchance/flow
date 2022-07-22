package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
)

func (p *Postgres) GetAPITokens(ctx context.Context) ([]*api.APIToken, error) {
	var apiTokens []*api.APIToken
	rows, err := p.db.QueryContext(ctx, "SELECT apitoken FROM apitokens;")
	if err != nil {
		if err == sql.ErrNoRows {
			return apiTokens, nil
		}
		return nil, err
	}

	for rows.Next() {
		var t *api.APIToken
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		apiTokens = append(apiTokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return apiTokens, nil
}

func (p *Postgres) GetAPIToken(ctx context.Context, token string) (*api.APIToken, error) {
	var apiToken *api.APIToken
	if err := p.db.QueryRowContext(ctx, "SELECT apitoken FROM apitokens WHERE apitoken @> json_build_object('token', $1::text)::jsonb;", token).Scan(&apiToken); err != nil {
		if err == sql.ErrNoRows {
			return nil, flow.ErrAPITokenDoesNotExist
		}
		return nil, err
	}
	return apiToken, nil
}

func (p *Postgres) CreateAPIToken(ctx context.Context, apiToken *api.APIToken) error {
	// set token fields
	apiToken.CreatedAt = time.Now()

	if _, err := p.db.Exec("INSERT INTO apitokens (apitoken) VALUES($1);", apiToken); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) UpdateAPIToken(ctx context.Context, apiToken *api.APIToken) error {
	if _, err := p.db.ExecContext(ctx, "UPDATE apitokens set apitoken = ($1) WHERE apitoken @> json_build_object('token', $2::text)::jsonb;", apiToken, apiToken.Token); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) DeleteAPIToken(ctx context.Context, token string) error {
	if _, err := p.db.ExecContext(ctx, "DELETE FROM apitokens WHERE apitoken @> json_build_object('token', $1::text)::jsonb;", token); err != nil {
		return err
	}

	return nil
}
