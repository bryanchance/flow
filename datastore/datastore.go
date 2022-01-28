package datastore

import (
	"git.underland.io/ehazlett/finca"
	"github.com/gogo/protobuf/jsonpb"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/go-redis/redis/v8"
)

var (
	dbPrefix = "finca"
)

type Datastore struct {
	storageClient *minio.Client
	redisClient   *redis.Client
	config        *finca.Config
}

func NewDatastore(cfg *finca.Config) (*Datastore, error) {
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
	rdb := redis.NewClient(redisOpts)

	return &Datastore{
		storageClient: mc,
		redisClient:   rdb,
		config:        cfg,
	}, nil
}

func (d *Datastore) Marshaler() *jsonpb.Marshaler {
	m := &jsonpb.Marshaler{
		EmitDefaults: true,
	}
	return m
}
