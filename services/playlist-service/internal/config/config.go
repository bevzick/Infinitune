package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/database"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/identity"
)

const (
	defaultHTTPHost           = "0.0.0.0"
	defaultHTTPPort           = "8080"
	defaultCatalogHost        = "127.0.0.1"
	defaultCatalogPort        = "9090"
	defaultPostgresHost       = "127.0.0.1"
	defaultPostgresPort       = "5432"
	defaultJWKSURL            = "http://auth-service:8080/v1/auth/jwks"
	defaultJWTIssuer          = "infinitune-auth"
	defaultJWTAudience        = "infinitune-api"
	defaultJWTKeyID           = "auth-key-1"
	defaultJWKSRefresh        = 5 * time.Minute
	defaultPersonalizationTTL = 5 * time.Minute
)

type Config struct {
	HTTPAddress           string
	CatalogAddress        string
	RecommendationAddress string
	PersonalizationTTL    time.Duration
	Database              database.Config
	Identity              identity.Config
	Origins               []string
}

func Load() (Config, error) {
	httpPort := envOrDefault("PLAYLIST_HTTP_PORT", defaultHTTPPort)
	if _, err := parsePort("PLAYLIST_HTTP_PORT", httpPort); err != nil {
		return Config{}, err
	}
	catalogPort := envOrDefault("CATALOG_GRPC_PORT", defaultCatalogPort)
	if _, err := parsePort("CATALOG_GRPC_PORT", catalogPort); err != nil {
		return Config{}, err
	}
	recommendationPort := envOrDefault("RECOMMENDATION_GRPC_PORT", defaultCatalogPort)
	if _, err := parsePort("RECOMMENDATION_GRPC_PORT", recommendationPort); err != nil {
		return Config{}, err
	}
	databaseConfig, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	origins, err := parseAllowedOrigins(envOrDefault("PLAYLIST_ALLOWED_ORIGINS", ""))
	if err != nil {
		return Config{}, err
	}
	personalizationTTL, err := parsePositiveDuration(
		"PLAYLIST_PERSONALIZATION_TTL",
		envOrDefault("PLAYLIST_PERSONALIZATION_TTL", defaultPersonalizationTTL.String()),
	)
	if err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress: net.JoinHostPort(envOrDefault("PLAYLIST_HTTP_HOST", defaultHTTPHost), httpPort),
		CatalogAddress: net.JoinHostPort(
			envOrDefault("CATALOG_GRPC_HOST", defaultCatalogHost), catalogPort,
		),
		RecommendationAddress: net.JoinHostPort(envOrDefault("RECOMMENDATION_GRPC_HOST", defaultCatalogHost), recommendationPort),
		PersonalizationTTL:    personalizationTTL,
		Database:              databaseConfig,
		Identity: identity.Config{
			JWKSURL:         envOrDefault("PLAYLIST_AUTH_JWKS_URL", defaultJWKSURL),
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
	origins := make([]string, 0)
	for rawOrigin := range strings.SplitSeq(value, ",") {
		origin := strings.TrimSpace(rawOrigin)
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid PLAYLIST_ALLOWED_ORIGINS entry %q", origin)
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins, nil
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
	name, err := requiredEnv("POSTGRES_PLAYLIST_DB")
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

func parsePositiveDuration(key, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid %s %q: expected a positive duration", key, value)
	}
	return duration, nil
}
