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
	"context"
	"strings"

	"github.com/go-redis/redis/v8"
)

func (q *Queue) Pull(ctx context.Context, namespace, queueName string) (*Task, error) {
	// pull based upon priority
	global := false
	switch strings.ToLower(namespace) {
	case "", "global":
		global = true
	}
	for _, p := range []Priority{URGENT, NORMAL, LOW} {
		k := getQueueName(namespace, queueName, p)
		nsKeys := []string{k}
		// if a global pull request get all queueName keys from all namespaces
		if global {
			gk := getQueueName("*", queueName, p)
			keys, err := q.redisClient.Keys(ctx, gk).Result()
			if err != nil {
				return nil, err
			}
			nsKeys = keys
		}
		for _, k := range nsKeys {
			data, err := q.redisClient.LPop(ctx, k).Bytes()
			if err != nil {
				if err != redis.Nil {
					return nil, err
				}
				continue
			}
			if data != nil {
				return &Task{
					Priority: p,
					Data:     data,
				}, nil
			}
		}
	}
	return nil, nil
}
