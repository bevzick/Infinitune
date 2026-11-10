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
)

type Config struct {
	HTTPAddress string
	Database    database.Config
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
	return Config{
		HTTPAddress: net.JoinHostPort(
			envOrDefault("RECOMMENDATION_HTTP_HOST", defaultHTTPHost),
			httpPort,
		),
		Database: databaseConfig,
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
