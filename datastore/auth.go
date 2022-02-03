package datastore

import (
	"context"
	"path"
	"time"

	"git.underland.io/ehazlett/fynca/pkg/auth"
	"github.com/pkg/errors"
)

// GetAuthenticatorKey is a helper func for authenticators to get authenticator specific keys
func (d *Datastore) GetAuthenticatorKey(ctx context.Context, a auth.Authenticator, key string) ([]byte, error) {
	authKey := getAuthenticatorKey(a.Name(), key)
	return d.redisClient.Get(ctx, authKey).Bytes()
}

// SetAuthenticatorKey is a helper func for authenticators to store authenticator specific data
func (d *Datastore) SetAuthenticatorKey(ctx context.Context, a auth.Authenticator, key string, value []byte, ttl time.Duration) error {
	authKey := getAuthenticatorKey(a.Name(), key)

	if err := d.redisClient.Set(ctx, authKey, value, ttl).Err(); err != nil {
		return errors.Wrapf(err, "error setting authenticator key %s for %s", key, a.Name())
	}

	return nil
}

func getAuthenticatorKey(authName string, key string) string {
	return path.Join(dbPrefix, "auth", authName, key)
}
