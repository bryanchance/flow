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
	"time"

	"github.com/ehazlett/flow"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"

	"github.com/go-redis/redis/v8"
)

const (
	serviceName = "datastore"
)

var (
	// ErrJobNotFound is returned when the specified job cannot be found
	ErrJobNotFound = errors.New("job not found")

	dbPrefix  = "flow"
	workerTTL = time.Second * 10
)

type Datastore struct {
	storageClient *minio.Client
	redisClient   *redis.Client
	config        *flow.Config
}

func NewDatastore(cfg *flow.Config) (*Datastore, error) {
	mc, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(cfg.S3AccessID, cfg.S3AccessKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, err
	}

	redisOpts, err := redis.ParseURL(cfg.DatabaseAddress)
	if err != nil {
		return nil, err
	}
	redisOpts.PoolSize = 256
	rdb := redis.NewClient(redisOpts)

	return &Datastore{
		storageClient: mc,
		redisClient:   rdb,
		config:        cfg,
	}, nil
}
