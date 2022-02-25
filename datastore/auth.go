// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package datastore

import (
	"context"
	"path"
	"time"

	"github.com/fynca/fynca/pkg/auth"
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
