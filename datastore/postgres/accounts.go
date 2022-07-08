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
	"database/sql"
	"fmt"
	"time"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/accounts/v1"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func (p *Postgres) GetAccounts(ctx context.Context) ([]*api.Account, error) {
	var accounts []*api.Account
	rows, err := p.db.QueryContext(ctx, "SELECT account FROM accounts;")
	if err != nil {
		if err == sql.ErrNoRows {
			return accounts, nil
		}
		return nil, err
	}

	for rows.Next() {
		var a *api.Account
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (p *Postgres) GetAccount(ctx context.Context, username string) (*api.Account, error) {
	var account *api.Account
	if err := p.db.QueryRowContext(ctx, "SELECT account FROM accounts WHERE account @> json_build_object('username', $1::text)::jsonb;", username).Scan(&account); err != nil {
		if err == sql.ErrNoRows {
			return nil, flow.ErrAccountDoesNotExist
		}
		return nil, err
	}
	return account, nil
}

func (p *Postgres) GetAccountByID(ctx context.Context, id string) (*api.Account, error) {
	var account *api.Account
	if err := p.db.QueryRowContext(ctx, "SELECT account FROM accounts WHERE account @> json_build_object('id', $1::text)::jsonb;", id).Scan(&account); err != nil {
		if err == sql.ErrNoRows {
			return nil, flow.ErrAccountDoesNotExist
		}
		return nil, err
	}
	return account, nil
}

func (p *Postgres) CreateAccount(ctx context.Context, account *api.Account) error {
	// validate
	if account.Username == "" {
		return fmt.Errorf("username cannot be blank")
	}

	// check for existing account
	if _, err := p.GetAccount(ctx, account.Username); err == nil {
		return flow.ErrAccountExists
	}

	if account.Password == "" {
		return fmt.Errorf("password cannot be blank")
	}

	// override id field to ensure unique
	account.ID = uuid.NewV4().String()
	// set account fields
	account.CreatedAt = time.Now()

	passwd := []byte(account.Password)
	account.Password = ""

	// bcrypt password
	passwordCrypt, err := bcrypt.GenerateFromPassword(passwd, bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	defer clearPassword(passwd)

	account.PasswordCrypt = passwordCrypt

	// create default user namespace
	nsID, err := p.CreateNamespace(ctx, &api.Namespace{
		Name:    account.Username,
		OwnerID: account.ID,
	})
	if err != nil {
		return errors.Wrapf(err, "error creating default namespace for %s in datastore", account.Username)
	}
	account.CurrentNamespace = nsID
	account.Namespaces = []string{nsID}

	if err := p.UpdateAccount(ctx, account); err != nil {
		return errors.Wrapf(err, "error creating user account for %s in datastore", account.Username)
	}

	logrus.Debugf("created account %s (admin: %v)", account.Username, account.Admin)
	if _, err := p.db.Exec("INSERT INTO accounts (account) VALUES($1)", account); err != nil {
		return err
	}
	return nil
}

func (p *Postgres) UpdateAccount(ctx context.Context, account *api.Account) error {
	if _, err := p.db.Exec("UPDATE accounts SET account = ($1) WHERE account @> json_build_object('id', $2::text)::jsonb;", account, account.ID); err != nil {
		return err
	}
	return nil
}

func (p *Postgres) ChangePassword(ctx context.Context, account *api.Account, password []byte) error {
	passwordCrypt, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	defer clearPassword(password)

	account.PasswordCrypt = passwordCrypt
	return p.UpdateAccount(ctx, account)
}

func (p *Postgres) DeleteAccount(ctx context.Context, username string) error {
	if _, err := p.db.ExecContext(ctx, "DELETE FROM accounts WHERE account @> json_build_object('username', $1::text)::jsonb;", username); err != nil {
		return err
	}

	return nil
}

func clearPassword(b []byte) {
	for i := 0; i < len(b); i++ {
		b[i] = 0
	}
}
