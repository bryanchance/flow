package datastore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (d *Datastore) SetCacheObject(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	cacheKey := getCacheKey(key)
	if err := d.redisClient.Set(ctx, cacheKey, value, ttl).Err(); err != nil {
		return errors.Wrapf(err, "error saving cache key %s in database", key)
	}
	return nil
}

func (d *Datastore) GetCacheObject(ctx context.Context, key string) ([]byte, error) {
	cacheKey := getCacheKey(key)
	data, err := d.redisClient.Get(ctx, cacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "error getting cache object for %s from database", key)
	}

	return data, nil
}

func (d *Datastore) ClearCacheObject(ctx context.Context, key string) error {
	cacheKey := getCacheKey(key)
	logrus.Debugf("clearing cache object %s", cacheKey)
	if err := d.redisClient.Del(ctx, cacheKey).Err(); err != nil {
		return err
	}
	return nil
}

func getCacheKey(key string) string {
	h := sha256.Sum256([]byte(key))
	k := hex.EncodeToString(h[:])
	return path.Join(dbPrefix, "cache", k)
}
