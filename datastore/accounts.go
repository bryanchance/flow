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
	"fmt"
	"path"
	"time"

	api "github.com/ehazlett/flow/api/services/accounts/v1"
	"github.com/go-redis/redis/v8"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrAccountExists is returned if an account already exists
	ErrAccountExists = errors.New("an account with that username already exists")
	// ErrAccountDoesNotExist is returned when an account cannot be found
	ErrAccountDoesNotExist = errors.New("account does not exist")
)

func (d *Datastore) GetAccounts(ctx context.Context) ([]*api.Account, error) {
	keys, err := d.redisClient.Keys(ctx, getAccountKey("*")).Result()
	if err != nil {
		return nil, err
	}

	accounts := []*api.Account{}
	for _, k := range keys {
		name := path.Base(k)
		account, err := d.GetAccount(ctx, name)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (d *Datastore) GetAccount(ctx context.Context, username string) (*api.Account, error) {
	accountKey := getAccountKey(username)
	data, err := d.redisClient.Get(ctx, accountKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrAccountDoesNotExist
		}
		return nil, errors.Wrapf(err, "error getting account %s from database", username)
	}

	account := api.Account{}
	if err := proto.Unmarshal(data, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

func (d *Datastore) GetAccountByID(ctx context.Context, id string) (*api.Account, error) {
	accountIDKey := getAccountIDKey(id)
	data, err := d.redisClient.Get(ctx, accountIDKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrAccountDoesNotExist
		}
		return nil, errors.Wrapf(err, "error getting account %s from database", id)
	}

	account := api.Account{}
	if err := proto.Unmarshal(data, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

func (d *Datastore) CreateAccount(ctx context.Context, account *api.Account) error {
	// validate
	if account.Username == "" {
		return fmt.Errorf("username cannot be blank")
	}

	// check for existing account
	if _, err := d.GetAccount(ctx, account.Username); err == nil {
		return ErrAccountExists
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
	nsID, err := d.CreateNamespace(ctx, &api.Namespace{
		Name:    account.Username,
		OwnerID: account.ID,
	})
	if err != nil {
		return errors.Wrapf(err, "error creating default namespace for %s in datastore", account.Username)
	}
	account.CurrentNamespace = nsID
	account.Namespaces = []string{nsID}

	if err := d.UpdateAccount(ctx, account); err != nil {
		return errors.Wrapf(err, "error creating user account for %s in datastore", account.Username)
	}

	logrus.Debugf("created account %s (admin: %v)", account.Username, account.Admin)
	return nil
}

func (d *Datastore) UpdateAccount(ctx context.Context, account *api.Account) error {
	accountKey := getAccountKey(account.Username)
	accountIDKey := getAccountIDKey(account.ID)

	data, err := proto.Marshal(account)
	if err != nil {
		return err
	}

	// save keys in both the account and account id lookup keyspaces
	for _, k := range []string{accountKey, accountIDKey} {
		if err := d.redisClient.Set(ctx, k, data, 0).Err(); err != nil {
			return errors.Wrapf(err, "error updating account for %s in database", account.Username)
		}
	}
	return nil
}

func (d *Datastore) ChangePassword(ctx context.Context, account *api.Account, password []byte) error {
	passwordCrypt, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	defer clearPassword(password)

	account.PasswordCrypt = passwordCrypt
	return d.UpdateAccount(ctx, account)
}

func (d *Datastore) DeleteAccount(ctx context.Context, username string) error {
	// ensure account exists
	acct, err := d.GetAccount(ctx, username)
	if err != nil {
		return err
	}

	accountKey := getAccountKey(username)
	accountIDKey := getAccountIDKey(acct.ID)
	for _, k := range []string{accountKey, accountIDKey} {
		if err := d.redisClient.Del(ctx, k).Err(); err != nil {
			return err
		}
	}

	return nil
}

func getAccountKey(username string) string {
	return path.Join(dbPrefix, "accounts", username)
}

func getAccountIDKey(id string) string {
	return path.Join(dbPrefix, "account-ids", id)
}

func clearPassword(b []byte) {
	for i := 0; i < len(b); i++ {
		b[i] = 0
	}
}
