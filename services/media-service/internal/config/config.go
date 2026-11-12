package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/database"
	"github.com/bevzick/Infinitune/services/media-service/internal/identity"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
)

const (
	defaultHTTPHost        = "0.0.0.0"
	defaultHTTPPort        = "8080"
	defaultGRPCHost        = "0.0.0.0"
	defaultGRPCPort        = "9090"
	defaultContentHost     = "127.0.0.1"
	defaultContentPort     = "9090"
	defaultPostgresHost    = "127.0.0.1"
	defaultPostgresPort    = "5432"
	defaultMinIOEndpoint   = "127.0.0.1:9000"
	defaultMinIOMediaName  = "infinitune-media"
	defaultJWKSURL         = "http://auth-service:8080/v1/auth/jwks"
	defaultJWTIssuer       = "infinitune-auth"
	defaultJWTAudience     = "infinitune-api"
	defaultJWTKeyID        = "auth-key-1"
	defaultJWKSRefresh     = 5 * time.Minute
	defaultKafkaBrokers    = "127.0.0.1:9092"
	defaultAssetReadyTopic = "media.asset.ready.v1"
	defaultOutboxPoll      = time.Second
	defaultOutboxBatch     = 100
)

type EventsConfig struct {
	Brokers         []string
	AssetReadyTopic string
	OutboxInterval  time.Duration
	OutboxBatch     int
}

type Config struct {
	HTTPAddress    string
	GRPCAddress    string
	ContentAddress string
	Database       database.Config
	Storage        storage.Config
	Identity       identity.Config
	Origins        []string
	Events         EventsConfig
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
	contentPort := envOrDefault("CONTENT_GRPC_PORT", defaultContentPort)
	if _, err := parsePort("CONTENT_GRPC_PORT", contentPort); err != nil {
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
	origins, err := parseAllowedOrigins(envOrDefault("MEDIA_ALLOWED_ORIGINS", ""))
	if err != nil {
		return Config{}, err
	}
	eventsConfig, err := loadEventsConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress: net.JoinHostPort(envOrDefault("MEDIA_HTTP_HOST", defaultHTTPHost), httpPort),
		GRPCAddress: net.JoinHostPort(envOrDefault("MEDIA_GRPC_HOST", defaultGRPCHost), grpcPort),
		ContentAddress: net.JoinHostPort(
			envOrDefault("CONTENT_GRPC_HOST", defaultContentHost), contentPort,
		),
		Database: databaseConfig,
		Storage:  storageConfig,
		Identity: identity.Config{
			JWKSURL:         envOrDefault("MEDIA_AUTH_JWKS_URL", defaultJWKSURL),
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
	topic := strings.TrimSpace(envOrDefault("KAFKA_MEDIA_ASSET_READY_TOPIC", defaultAssetReadyTopic))
	if topic == "" {
		return EventsConfig{}, fmt.Errorf("Kafka media asset ready topic must not be blank")
	}
	interval := defaultOutboxPoll
	if raw := os.Getenv("MEDIA_OUTBOX_POLL_INTERVAL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return EventsConfig{}, fmt.Errorf("invalid MEDIA_OUTBOX_POLL_INTERVAL %q", raw)
		}
		interval = parsed
	}
	batch := defaultOutboxBatch
	if raw := os.Getenv("MEDIA_OUTBOX_BATCH_SIZE"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000 {
			return EventsConfig{}, fmt.Errorf("invalid MEDIA_OUTBOX_BATCH_SIZE %q: expected 1..1000", raw)
		}
		batch = parsed
	}
	return EventsConfig{Brokers: brokers, AssetReadyTopic: topic, OutboxInterval: interval, OutboxBatch: batch}, nil
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
			return nil, fmt.Errorf("invalid MEDIA_ALLOWED_ORIGINS entry %q", origin)
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
