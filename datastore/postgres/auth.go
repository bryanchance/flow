package postgres

import (
	"context"
	"time"

	"github.com/ehazlett/flow/pkg/auth"
)

func (p *Postgres) GetAuthenticatorKey(ctx context.Context, a auth.Authenticator, key string) ([]byte, error) {
	var val []byte
	if err := p.db.QueryRow("SELECT value FROM authenticator WHERE key = ($1)", key).Scan(&val); err != nil {
		return nil, err
	}
	return val, nil
}

func (p *Postgres) SetAuthenticatorKey(ctx context.Context, a auth.Authenticator, key string, value []byte, ttl time.Duration) error {
	if _, err := p.db.Exec("INSERT INTO authenticator (key, value) VALUES($1, $2)", key, value); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) GetAuthenticatorKeys(ctx context.Context, a auth.Authenticator, prefix string) ([][]byte, error) {
	var keys [][]byte
	if err := p.db.QueryRow("SELECT value FROM authenticator;").Scan(&keys); err != nil {
		return nil, err
	}

	return keys, nil
}

func (p *Postgres) DeleteAuthenticatorKey(ctx context.Context, key string) error {
	if _, err := p.db.ExecContext(ctx, "DELETE FROM authenticator WHERE key = ($1)", key); err != nil {
		return err
	}

	return nil
}
