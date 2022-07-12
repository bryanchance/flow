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
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

const (
	accountsTableName  = "accounts"
	workflowsTableName = "workflows"
	maxRetries         = 10
)

type Postgres struct {
	db        *sql.DB
	queueLock *sync.Mutex
}

func NewPostgres(addr string) (*Postgres, error) {
	for i := 0; i < maxRetries; i++ {
		db, err := sql.Open("postgres", addr)
		if err != nil {
			return nil, err
		}
		if err := db.Ping(); err != nil {
			logrus.WithError(err).Warnf("unable to reach postgres at %s", addr)
			time.Sleep(time.Second * 1)
			continue
		}
		return &Postgres{
			db:        db,
			queueLock: &sync.Mutex{},
		}, nil
	}

	return nil, fmt.Errorf("max retries failed (%d) attempting to connect to postgres at %s", maxRetries, addr)
}
