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
