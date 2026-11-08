package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/bevzick/Infinitune/services/media-service/internal/database"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
)

const (
	defaultHTTPHost       = "0.0.0.0"
	defaultHTTPPort       = "8080"
	defaultGRPCHost       = "0.0.0.0"
	defaultGRPCPort       = "9090"
	defaultPostgresHost   = "127.0.0.1"
	defaultPostgresPort   = "5432"
	defaultMinIOEndpoint  = "127.0.0.1:9000"
	defaultMinIOMediaName = "infinitune-media"
)

type Config struct {
	HTTPAddress string
	GRPCAddress string
	Database    database.Config
	Storage     storage.Config
}

func Load() (Config, error) {
	httpPort := envOrDefault("MEDIA_HTTP_PORT", defaultHTTPPort)
	if _, err := parsePort("MEDIA_HTTP_PORT", httpPort); err != nil {
		return Config{}, err
	}
	grpcPort := envOrDefault("MEDIA_GRPC_PORT", defaultGRPCPort)
	if _, err := parsePort("MEDIA_GRPC_PORT", grpcPort); err != nil {
		return Config{}, err
	}
	databaseConfig, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	storageConfig, err := loadStorageConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress: net.JoinHostPort(envOrDefault("MEDIA_HTTP_HOST", defaultHTTPHost), httpPort),
		GRPCAddress: net.JoinHostPort(envOrDefault("MEDIA_GRPC_HOST", defaultGRPCHost), grpcPort),
		Database:    databaseConfig,
		Storage:     storageConfig,
	}, nil
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
	name, err := requiredEnv("POSTGRES_MEDIA_DB")
	if err != nil {
		return database.Config{}, err
	}
	return database.Config{
		Host: envOrDefault("POSTGRES_HOST", defaultPostgresHost), Port: port,
		User: user, Password: password, Name: name,
	}, nil
}

func loadStorageConfig() (storage.Config, error) {
	accessKey, err := requiredEnv("MINIO_ROOT_USER")
	if err != nil {
		return storage.Config{}, err
	}
	secretKey, err := requiredEnv("MINIO_ROOT_PASSWORD")
	if err != nil {
		return storage.Config{}, err
	}
	secure, err := strconv.ParseBool(envOrDefault("MINIO_USE_SSL", "false"))
	if err != nil {
		return storage.Config{}, fmt.Errorf("invalid MINIO_USE_SSL: %w", err)
	}
	endpoint := envOrDefault("MINIO_ENDPOINT", defaultMinIOEndpoint)
	if strings.Contains(endpoint, "://") {
		return storage.Config{}, fmt.Errorf("MINIO_ENDPOINT must not include a URL scheme")
	}
	return storage.Config{
		Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey,
		Bucket: envOrDefault("MINIO_MEDIA_BUCKET", defaultMinIOMediaName), Secure: secure,
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
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func parsePort(key, value string) (uint16, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid %s %q: expected a number from 1 to 65535", key, value)
	}
	return uint16(port), nil
}
