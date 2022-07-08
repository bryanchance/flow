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
	"log"
	"sync"

	_ "github.com/lib/pq"
)

const (
	accountsTableName  = "accounts"
	workflowsTableName = "workflows"
)

type Postgres struct {
	db        *sql.DB
	queueLock *sync.Mutex
}

func NewPostgres(addr string) (*Postgres, error) {
	db, err := sql.Open("postgres", addr)
	if err != nil {
		log.Fatal(err)
	}

	return &Postgres{
		db:        db,
		queueLock: &sync.Mutex{},
	}, nil
}
