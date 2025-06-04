package redisx

import (
	"context"
	"fmt" // Added for error formatting
	"log" // Added for logging
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/phuhao00/pandaparty/config" // Added for config.RedisConfig
)

type RedisClient struct {
	client *redis.Client // This can be either a standard client or a failover client
}

func (r *RedisClient) GetReal() *redis.Client {
	return r.client
}

func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) {
	if cfg.MasterName != "" && len(cfg.SentinelAddrs) > 0 {
		// Sentinel setup
		failoverOpts := &redis.FailoverOptions{
			MasterName:    cfg.MasterName,
			SentinelAddrs: cfg.SentinelAddrs,
			Password:      cfg.Password, // Will be empty if not set, which is fine
			DB:            cfg.DB,       // Will be 0 if not set, which is fine
		}
		rdb := redis.NewFailoverClient(failoverOpts)
		log.Printf("RedisClient: Initializing with Sentinel config (Master: %s)", cfg.MasterName)
		return &RedisClient{client: rdb}, nil
	} else if cfg.Addr != "" {
		// Single-node setup
		opts := &redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password, // Will be empty if not set
			DB:       cfg.DB,       // Will be 0 if not set
		}
		rdb := redis.NewClient(opts)
		log.Printf("RedisClient: Initializing with single-node config (Addr: %s)", cfg.Addr)
		return &RedisClient{client: rdb}, nil
	} else {
		// Insufficient configuration
		return nil, fmt.Errorf("redis configuration is insufficient: either master_name and sentinel_addrs must be provided, or a single addr must be specified")
	}
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
