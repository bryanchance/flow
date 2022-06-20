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
package queue

import (
	"fmt"

	"github.com/go-redis/redis/v8"
)

type Priority string

const (
	UNKNOWN Priority = "unknown"
	LOW     Priority = "low"
	NORMAL  Priority = "normal"
	URGENT  Priority = "urgent"
)

var (
	dbPrefix = "fynca"
)

type Task struct {
	Priority Priority
	Data     []byte
}

type Queue struct {
	redisClient *redis.Client
}

func NewQueue(redisAddr string) (*Queue, error) {
	redisOpts, err := redis.ParseURL(redisAddr)
	if err != nil {
		return nil, err
	}
	redisOpts.PoolSize = 64
	rdb := redis.NewClient(redisOpts)

	return &Queue{
		redisClient: rdb,
	}, nil
}

func getQueueName(name string, priority Priority) string {
	return fmt.Sprintf("%s/queue/%s.%s", dbPrefix, name, priority)
}
