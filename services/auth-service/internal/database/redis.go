package database

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string
	Port     uint16
	Password string
	DB       int
}

type RedisClient struct {
	*redis.Client
}

func OpenRedis(ctx context.Context, config RedisConfig) (*RedisClient, error) {
	client := &RedisClient{Client: redis.NewClient(&redis.Options{
		Addr:     net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.Port), 10)),
		Password: config.Password,
		DB:       config.DB,
	})}

	if err := client.Client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}

func (c *RedisClient) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx).Err()
}
