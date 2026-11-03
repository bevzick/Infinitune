package config

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/bevzick/Infinitune/services/auth-service/internal/database"
	"github.com/bevzick/Infinitune/services/auth-service/internal/password"
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
	Password    password.Params
}

func Load() (Config, error) {
	host := envOrDefault("AUTH_HTTP_HOST", defaultHTTPHost)
	port := envOrDefault("AUTH_HTTP_PORT", defaultHTTPPort)

	if _, err := parsePort("AUTH_HTTP_PORT", port); err != nil {
		return Config{}, err
	}

	databaseConfig, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	passwordParams, err := loadPasswordParams()
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddress: net.JoinHostPort(host, port),
		Database:    databaseConfig,
		Password:    passwordParams,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}

func loadDatabaseConfig() (database.Config, error) {
	port, err := parsePort("POSTGRES_PORT", envOrDefault("POSTGRES_PORT", defaultPostgresPort))
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
	name, err := requiredEnv("POSTGRES_AUTH_DB")
	if err != nil {
		return database.Config{}, err
	}

	return database.Config{
		Host:     envOrDefault("POSTGRES_HOST", defaultPostgresHost),
		Port:     port,
		User:     user,
		Password: password,
		Name:     name,
	}, nil
}

func requiredEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}

	return value, nil
}

func loadPasswordParams() (password.Params, error) {
	params := password.DefaultParams()

	memory, err := parseUintEnv("AUTH_PASSWORD_MEMORY_KIB", uint64(params.MemoryKiB), 32)
	if err != nil {
		return password.Params{}, err
	}
	iterations, err := parseUintEnv("AUTH_PASSWORD_ITERATIONS", uint64(params.Iterations), 32)
	if err != nil {
		return password.Params{}, err
	}
	parallelism, err := parseUintEnv("AUTH_PASSWORD_PARALLELISM", uint64(params.Parallelism), 8)
	if err != nil {
		return password.Params{}, err
	}

	params.MemoryKiB = uint32(memory)
	params.Iterations = uint32(iterations)
	params.Parallelism = uint8(parallelism)
	if err = params.Validate(); err != nil {
		return password.Params{}, fmt.Errorf("invalid password hashing configuration: %w", err)
	}

	return params, nil
}

func parseUintEnv(key string, fallback uint64, bitSize int) (uint64, error) {
	value := envOrDefault(key, strconv.FormatUint(fallback, 10))
	parsed, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: expected an unsigned integer", key, value)
	}

	return parsed, nil
}

func parsePort(key, port string) (uint16, error) {
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return 0, fmt.Errorf("invalid %s %q: expected a number from 1 to 65535", key, port)
	}

	return uint16(portNumber), nil
}
