package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/database"
	"github.com/bevzick/Infinitune/services/comment-service/internal/identity"
)

const (
	defaultHTTPHost     = "0.0.0.0"
	defaultHTTPPort     = "8080"
	defaultGRPCHost     = "0.0.0.0"
	defaultGRPCPort     = "9090"
	defaultCatalogHost  = "127.0.0.1"
	defaultCatalogPort  = "9090"
	defaultPostgresHost = "127.0.0.1"
	defaultPostgresPort = "5432"
	defaultJWKSURL      = "http://auth-service:8080/v1/auth/jwks"
	defaultJWTIssuer    = "infinitune-auth"
	defaultJWTAudience  = "infinitune-api"
	defaultJWTKeyID     = "auth-key-1"
	defaultJWKSRefresh  = 5 * time.Minute
)

type Config struct {
	HTTPAddress    string
	GRPCAddress    string
	CatalogAddress string
	Database       database.Config
	Identity       identity.Config
	Origins        []string
}

func Load() (Config, error) {
	httpPort := envOrDefault("COMMENT_HTTP_PORT", defaultHTTPPort)
	if _, err := parsePort("COMMENT_HTTP_PORT", httpPort); err != nil {
		return Config{}, err
	}
	grpcPort := envOrDefault("COMMENT_GRPC_PORT", defaultGRPCPort)
	if _, err := parsePort("COMMENT_GRPC_PORT", grpcPort); err != nil {
		return Config{}, err
	}
	catalogPort := envOrDefault("CATALOG_GRPC_PORT", defaultCatalogPort)
	if _, err := parsePort("CATALOG_GRPC_PORT", catalogPort); err != nil {
		return Config{}, err
	}
	databaseConfig, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	origins, err := parseAllowedOrigins(envOrDefault("COMMENT_ALLOWED_ORIGINS", ""))
	if err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress: net.JoinHostPort(
			envOrDefault("COMMENT_HTTP_HOST", defaultHTTPHost), httpPort,
		),
		GRPCAddress: net.JoinHostPort(
			envOrDefault("COMMENT_GRPC_HOST", defaultGRPCHost), grpcPort,
		),
		CatalogAddress: net.JoinHostPort(
			envOrDefault("CATALOG_GRPC_HOST", defaultCatalogHost), catalogPort,
		),
		Database: databaseConfig,
		Identity: identity.Config{
			JWKSURL:         envOrDefault("COMMENT_AUTH_JWKS_URL", defaultJWKSURL),
			Issuer:          envOrDefault("AUTH_JWT_ISSUER", defaultJWTIssuer),
			Audience:        envOrDefault("AUTH_JWT_AUDIENCE", defaultJWTAudience),
			KeyID:           envOrDefault("AUTH_JWT_KEY_ID", defaultJWTKeyID),
			RefreshInterval: defaultJWKSRefresh,
		},
		Origins: origins,
	}, nil
}

func parseAllowedOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for raw := range strings.SplitSeq(value, ",") {
		origin := strings.TrimSpace(raw)
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid COMMENT_ALLOWED_ORIGINS entry %q", origin)
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		result = append(result, origin)
	}
	return result, nil
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
	name, err := requiredEnv("POSTGRES_COMMENT_DB")
	if err != nil {
		return database.Config{}, err
	}
	return database.Config{
		Host: envOrDefault("POSTGRES_HOST", defaultPostgresHost), Port: port,
		User: user, Password: password, Name: name,
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
