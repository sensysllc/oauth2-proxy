package postgres

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v9"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
)

type RedisStore struct {
	configStore  ConfigStore
	redisOptions *options.Redis
	rdb          *redis.Client
}

func NewRedisStore(opts options.Redis, configStore ConfigStore) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
	})

	rs := &RedisStore{
		configStore:  configStore,
		redisOptions: &opts,
		rdb:          rdb,
	}
	return rs, nil

}

func (rs *RedisStore) key(id string) string {
	return rs.redisOptions.Prefix + "-" + id
}

func (rs *RedisStore) Create(ctx context.Context, id string, providerConfig []byte) error {
	// create in postgres
	err := rs.configStore.Create(ctx, id, providerConfig)
	if err != nil {
		return err
	}

	// then set(create) in redis store
	err = rs.rdb.Set(ctx, rs.key(id), providerConfig, rs.redisOptions.Expiry).Err()
	if err != nil {
		return err
	}
	return nil
}

func (rs *RedisStore) Update(ctx context.Context, id string, providerconf []byte) error {
	// update in postgres
	err := rs.configStore.Update(ctx, id, providerconf)
	if err != nil {
		return err
	}

	// update in redis store
	err = rs.rdb.Set(ctx, rs.key(id), providerconf, rs.redisOptions.Expiry).Err()
	if err != nil {
		return err
	}
	return nil
}

func (rs *RedisStore) Get(ctx context.Context, id string) (string, error) {
	// check if redis store has key value pair
	val, err := rs.rdb.Get(ctx, rs.key(id)).Result()
	if err != nil {
		// check in postgres store
		val, err := rs.configStore.Get(ctx, id)
		if err != nil {
			return "", fmt.Errorf("unable to get from postgres store: %v", err)
		}

		// set or create entry in redis store
		err = rs.rdb.Set(ctx, rs.key(id), val, rs.redisOptions.Expiry).Err()
		if err != nil {
			return "", fmt.Errorf("could not create entry in redis store: %v", err)
		}
		return val, nil
	}

	return val, nil
}

func (rs *RedisStore) Delete(ctx context.Context, id string) error {
	// delete in postgres store
	err := rs.configStore.Delete(ctx, id)
	if err != nil {
		return err
	}

	// delete in redis store
	err = rs.rdb.Del(ctx, rs.key(id)).Err()
	if err != nil {
		return err
	}
	return nil
}
