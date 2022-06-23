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
)

var (
	// ErrNamespaceDoesNotExist is returned when an namespace cannot be found
	ErrNamespaceDoesNotExist = errors.New("namespace does not exist")
)

func (d *Datastore) GetNamespaces(ctx context.Context) ([]*api.Namespace, error) {
	keys, err := d.redisClient.Keys(ctx, getNamespaceKey("*")).Result()
	if err != nil {
		return nil, err
	}

	namespaces := []*api.Namespace{}
	for _, k := range keys {
		name := path.Base(k)
		namespace, err := d.GetNamespace(ctx, name)
		if err != nil {
			return nil, err
		}
		namespaces = append(namespaces, namespace)
	}

	return namespaces, nil
}

func (d *Datastore) GetNamespace(ctx context.Context, id string) (*api.Namespace, error) {
	namespaceKey := getNamespaceKey(id)
	data, err := d.redisClient.Get(ctx, namespaceKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrNamespaceDoesNotExist
		}
		return nil, errors.Wrapf(err, "error getting namespace %s from database", id)
	}

	namespace := api.Namespace{}
	if err := proto.Unmarshal(data, &namespace); err != nil {
		return nil, err
	}
	return &namespace, nil
}

func (d *Datastore) CreateNamespace(ctx context.Context, namespace *api.Namespace) (string, error) {
	id := uuid.NewV4().String()

	namespaceKey := getNamespaceKey(id)

	// validate
	if namespace.Name == "" {
		return "", fmt.Errorf("name cannot be blank")
	}

	// override id field to ensure unique
	namespace.ID = id
	// set namespace fields
	namespace.CreatedAt = time.Now()

	data, err := proto.Marshal(namespace)
	if err != nil {
		return "", err
	}

	if err := d.redisClient.Set(ctx, namespaceKey, data, 0).Err(); err != nil {
		return "", errors.Wrapf(err, "error updating namespace for %s in database", namespace.Name)
	}

	logrus.Debugf("created namespace %s (%s))", namespace.Name, namespace.ID)
	return namespace.ID, nil
}

func (d *Datastore) UpdateNamespace(ctx context.Context, namespace *api.Namespace) error {
	namespaceKey := getNamespaceKey(namespace.ID)

	data, err := proto.Marshal(namespace)
	if err != nil {
		return err
	}
	if err := d.redisClient.Set(ctx, namespaceKey, data, 0).Err(); err != nil {
		return errors.Wrapf(err, "error updating namespace for %s in database", namespace.ID)
	}
	return nil
}

func (d *Datastore) DeleteNamespace(ctx context.Context, id string) error {
	// ensure namespace exists
	if _, err := d.GetNamespace(ctx, id); err != nil {
		return err
	}

	namespaceKey := getNamespaceKey(id)
	if err := d.redisClient.Del(ctx, namespaceKey).Err(); err != nil {
		return err
	}

	return nil
}

func getNamespaceKey(id string) string {
	return path.Join(dbPrefix, "namespaces", id)
}
