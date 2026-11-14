package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host     string
	Port     uint16
	User     string
	Password string
	Name     string
}

func Open(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("parse postgres configuration: %w", err)
	}

	poolConfig.ConnConfig.Host = config.Host
	poolConfig.ConnConfig.Port = config.Port
	poolConfig.ConnConfig.User = config.User
	poolConfig.ConnConfig.Password = config.Password
	poolConfig.ConnConfig.Database = config.Name

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
