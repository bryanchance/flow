package datastore

import (
	"git.underland.io/ehazlett/finca"
	"github.com/gogo/protobuf/jsonpb"
	minio "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
)

type Datastore struct {
	storageClient *minio.Client
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

	return &Datastore{
		storageClient: mc,
		config:        cfg,
	}, nil
}

func (d *Datastore) Marshaler() *jsonpb.Marshaler {
	m := &jsonpb.Marshaler{
		EmitDefaults: true,
	}
	return m
}
