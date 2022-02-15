package datastore

import (
	"context"
	"fmt"
	"path"
	"time"

	api "git.underland.io/ehazlett/fynca/api/services/accounts/v1"
	"github.com/go-redis/redis/v8"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

var (
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

func (d *Datastore) CreateAccount(ctx context.Context, account *api.Account) error {
	accountKey := getAccountKey(account.Username)

	// validate
	if account.Username == "" {
		return fmt.Errorf("username cannot be blank")
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

	data, err := proto.Marshal(account)
	if err != nil {
		return err
	}

	if err := d.redisClient.Set(ctx, accountKey, data, 0).Err(); err != nil {
		return errors.Wrapf(err, "error updating account for %s in database", account.Username)
	}

	logrus.Debugf("created account %s (admin: %v)", account.Username, account.Admin)
	return nil
}

func (d *Datastore) UpdateAccount(ctx context.Context, account *api.Account) error {
	accountKey := getAccountKey(account.Username)

	data, err := proto.Marshal(account)
	if err != nil {
		return err
	}
	if err := d.redisClient.Set(ctx, accountKey, data, 0).Err(); err != nil {
		return errors.Wrapf(err, "error updating account for %s in database", account.Username)
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
	if _, err := d.GetAccount(ctx, username); err != nil {
		return err
	}

	accountKey := getAccountKey(username)
	if err := d.redisClient.Del(ctx, accountKey).Err(); err != nil {
		return err
	}

	return nil
}

func getAccountKey(username string) string {
	return path.Join(dbPrefix, "accounts", username)
}

func clearPassword(b []byte) {
	for i := 0; i < len(b); i++ {
		b[i] = 0
	}
}
