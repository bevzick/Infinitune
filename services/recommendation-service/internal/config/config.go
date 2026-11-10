package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/database"
)

const (
	defaultHTTPHost     = "0.0.0.0"
	defaultHTTPPort     = "8080"
	defaultPostgresHost = "127.0.0.1"
	defaultPostgresPort = "5432"
	defaultGRPCHost     = "127.0.0.1"
	defaultGRPCPort     = "9090"
)

type Config struct {
	HTTPAddress        string
	UserAddress        string
	InteractionAddress string
	HistoryAddress     string
	CatalogAddress     string
	ContentAddress     string
	Database           database.Config
}

func Load() (Config, error) {
	httpPort := envOrDefault("RECOMMENDATION_HTTP_PORT", defaultHTTPPort)
	if _, err := parsePort("RECOMMENDATION_HTTP_PORT", httpPort); err != nil {
		return Config{}, err
	}
	databaseConfig, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	grpcPort := envOrDefault("USER_GRPC_PORT", defaultGRPCPort)
	if _, err = parsePort("USER_GRPC_PORT", grpcPort); err != nil {
		return Config{}, err
	}
	interactionPort := envOrDefault("INTERACTION_GRPC_PORT", defaultGRPCPort)
	if _, err = parsePort("INTERACTION_GRPC_PORT", interactionPort); err != nil {
		return Config{}, err
	}
	historyPort := envOrDefault("HISTORY_GRPC_PORT", defaultGRPCPort)
	if _, err = parsePort("HISTORY_GRPC_PORT", historyPort); err != nil {
		return Config{}, err
	}
	catalogPort := envOrDefault("CATALOG_GRPC_PORT", defaultGRPCPort)
	if _, err = parsePort("CATALOG_GRPC_PORT", catalogPort); err != nil {
		return Config{}, err
	}
	contentPort := envOrDefault("CONTENT_GRPC_PORT", defaultGRPCPort)
	if _, err = parsePort("CONTENT_GRPC_PORT", contentPort); err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress: net.JoinHostPort(
			envOrDefault("RECOMMENDATION_HTTP_HOST", defaultHTTPHost),
			httpPort,
		),
		UserAddress:        net.JoinHostPort(envOrDefault("USER_GRPC_HOST", defaultGRPCHost), grpcPort),
		InteractionAddress: net.JoinHostPort(envOrDefault("INTERACTION_GRPC_HOST", defaultGRPCHost), interactionPort),
		HistoryAddress:     net.JoinHostPort(envOrDefault("HISTORY_GRPC_HOST", defaultGRPCHost), historyPort),
		CatalogAddress:     net.JoinHostPort(envOrDefault("CATALOG_GRPC_HOST", defaultGRPCHost), catalogPort),
		ContentAddress:     net.JoinHostPort(envOrDefault("CONTENT_GRPC_HOST", defaultGRPCHost), contentPort),
		Database:           databaseConfig,
	}, nil
}

func loadDatabaseConfig() (database.Config, error) {
	port, err := parsePort(
		"POSTGRES_PORT",
		envOrDefault("POSTGRES_PORT", defaultPostgresPort),
	)
	if err != nil {
		return database.Config{}, err
	}
	user, err := requiredEnv("POSTGRES_USER")
	if err != nil {
		return database.Config{}, err
	}
	password, err := requiredEnv("POSTGRES_PASSWORD")
	if err != nil {
		return database.Config{}, err
	}
	name, err := requiredEnv("POSTGRES_RECOMMENDATION_DB")
	if err != nil {
		return database.Config{}, err
	}
	return database.Config{
		Host: envOrDefault("POSTGRES_HOST", defaultPostgresHost),
		Port: port, User: user, Password: password, Name: name,
	}, nil
}

func requiredEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parsePort(key, value string) (uint16, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf(
			"invalid %s %q: expected a number from 1 to 65535",
			key,
			value,
		)
	}
	return uint16(port), nil
}
