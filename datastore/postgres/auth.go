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
