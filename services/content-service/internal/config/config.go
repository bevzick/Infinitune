package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/database"
	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
)

const (
	defaultHTTPHost           = "0.0.0.0"
	defaultHTTPPort           = "8080"
	defaultGRPCHost           = "0.0.0.0"
	defaultGRPCPort           = "9090"
	defaultUserHost           = "127.0.0.1"
	defaultUserPort           = "9090"
	defaultMediaHost          = "127.0.0.1"
	defaultMediaPort          = "9090"
	defaultCatalogHost        = "127.0.0.1"
	defaultCatalogPort        = "9090"
	defaultPostgresHost       = "127.0.0.1"
	defaultPostgresPort       = "5432"
	defaultJWKSURL            = "http://auth-service:8080/v1/auth/jwks"
	defaultJWTIssuer          = "infinitune-auth"
	defaultJWTAudience        = "infinitune-api"
	defaultJWTKeyID           = "auth-key-1"
	defaultJWKSRefresh        = 5 * time.Minute
	defaultKafkaBrokers       = "127.0.0.1:9092"
	defaultMediaReadyTopic    = "media.asset.ready.v1"
	defaultMediaConsumerGroup = "content-media-v1"
)

type EventsConfig struct {
	Brokers            []string
	MediaReadyTopic    string
	MediaConsumerGroup string
}

type Config struct {
	HTTPAddress    string
	GRPCAddress    string
	UserAddress    string
	MediaAddress   string
	CatalogAddress string
	Database       database.Config
	Identity       identity.Config
	Origins        []string
	Events         EventsConfig
}

func Load() (Config, error) {
	httpPort := envOrDefault("CONTENT_HTTP_PORT", defaultHTTPPort)
	if _, err := parsePort("CONTENT_HTTP_PORT", httpPort); err != nil {
		return Config{}, err
	}
	grpcPort := envOrDefault("CONTENT_GRPC_PORT", defaultGRPCPort)
	if _, err := parsePort("CONTENT_GRPC_PORT", grpcPort); err != nil {
		return Config{}, err
	}
	userPort := envOrDefault("USER_GRPC_PORT", defaultUserPort)
	if _, err := parsePort("USER_GRPC_PORT", userPort); err != nil {
		return Config{}, err
	}
	mediaPort := envOrDefault("MEDIA_GRPC_PORT", defaultMediaPort)
	if _, err := parsePort("MEDIA_GRPC_PORT", mediaPort); err != nil {
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
	origins, err := parseAllowedOrigins(envOrDefault("CONTENT_ALLOWED_ORIGINS", ""))
	if err != nil {
		return Config{}, err
	}
	eventsConfig, err := loadEventsConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress:    net.JoinHostPort(envOrDefault("CONTENT_HTTP_HOST", defaultHTTPHost), httpPort),
		GRPCAddress:    net.JoinHostPort(envOrDefault("CONTENT_GRPC_HOST", defaultGRPCHost), grpcPort),
		UserAddress:    net.JoinHostPort(envOrDefault("USER_GRPC_HOST", defaultUserHost), userPort),
		MediaAddress:   net.JoinHostPort(envOrDefault("MEDIA_GRPC_HOST", defaultMediaHost), mediaPort),
		CatalogAddress: net.JoinHostPort(envOrDefault("CATALOG_GRPC_HOST", defaultCatalogHost), catalogPort),
		Database:       databaseConfig,
		Identity: identity.Config{
			JWKSURL:         envOrDefault("CONTENT_AUTH_JWKS_URL", defaultJWKSURL),
			Issuer:          envOrDefault("AUTH_JWT_ISSUER", defaultJWTIssuer),
			Audience:        envOrDefault("AUTH_JWT_AUDIENCE", defaultJWTAudience),
			KeyID:           envOrDefault("AUTH_JWT_KEY_ID", defaultJWTKeyID),
			RefreshInterval: defaultJWKSRefresh,
		},
		Origins: origins,
		Events:  eventsConfig,
	}, nil
}

func loadEventsConfig() (EventsConfig, error) {
	rawBrokers := strings.Split(envOrDefault("KAFKA_BROKERS", defaultKafkaBrokers), ",")
	brokers := make([]string, 0, len(rawBrokers))
	for _, raw := range rawBrokers {
		broker := strings.TrimSpace(raw)
		if broker == "" {
			return EventsConfig{}, fmt.Errorf("KAFKA_BROKERS contains an empty broker")
		}
		if _, _, err := net.SplitHostPort(broker); err != nil {
			return EventsConfig{}, fmt.Errorf("invalid KAFKA_BROKERS entry %q", broker)
		}
		brokers = append(brokers, broker)
	}
	topic := strings.TrimSpace(envOrDefault("KAFKA_MEDIA_ASSET_READY_TOPIC", defaultMediaReadyTopic))
	group := strings.TrimSpace(envOrDefault("KAFKA_CONTENT_MEDIA_GROUP", defaultMediaConsumerGroup))
	if topic == "" || group == "" {
		return EventsConfig{}, fmt.Errorf("Kafka media topic and consumer group must not be blank")
	}
	return EventsConfig{Brokers: brokers, MediaReadyTopic: topic, MediaConsumerGroup: group}, nil
}

func parseAllowedOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	seen := make(map[string]struct{})
	origins := make([]string, 0)
	for raw := range strings.SplitSeq(value, ",") {
		origin := strings.TrimSpace(raw)
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid CONTENT_ALLOWED_ORIGINS entry %q", origin)
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
	name, err := requiredEnv("POSTGRES_CONTENT_DB")
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
