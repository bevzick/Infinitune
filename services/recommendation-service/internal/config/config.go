package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/database"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/identity"
)

const (
	defaultHTTPHost            = "0.0.0.0"
	defaultHTTPPort            = "8080"
	defaultPostgresHost        = "127.0.0.1"
	defaultPostgresPort        = "5432"
	defaultGRPCHost            = "127.0.0.1"
	defaultGRPCPort            = "9090"
	defaultRunTTL              = time.Hour
	defaultJWKSURL             = "http://auth-service:8080/v1/auth/jwks"
	defaultJWTIssuer           = "infinitune-auth"
	defaultJWTAudience         = "infinitune-api"
	defaultJWTKeyID            = "auth-key-1"
	defaultJWKSRefresh         = 5 * time.Minute
	defaultKafkaBrokers        = "127.0.0.1:9092"
	defaultHistoryTopic        = "history.listening.recorded.v1"
	defaultHistoryClearedTopic = "history.listening.cleared.v1"
	defaultConsumerGroup       = "recommendation-history-v1"
)

type EventsConfig struct {
	Brokers             []string
	HistoryTopic        string
	HistoryClearedTopic string
	ConsumerGroup       string
}

type Config struct {
	HTTPAddress        string
	GRPCAddress        string
	UserAddress        string
	InteractionAddress string
	HistoryAddress     string
	CatalogAddress     string
	ContentAddress     string
	RunTTL             time.Duration
	Database           database.Config
	Identity           identity.Config
	Origins            []string
	Events             EventsConfig
}

func Load() (Config, error) {
	httpPort := envOrDefault("RECOMMENDATION_HTTP_PORT", defaultHTTPPort)
	if _, err := parsePort("RECOMMENDATION_HTTP_PORT", httpPort); err != nil {
		return Config{}, err
	}
	recommendationGRPCPort := envOrDefault("RECOMMENDATION_GRPC_PORT", defaultGRPCPort)
	if _, err := parsePort("RECOMMENDATION_GRPC_PORT", recommendationGRPCPort); err != nil {
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
	runTTL, err := parsePositiveDuration("RECOMMENDATION_RUN_TTL", envOrDefault("RECOMMENDATION_RUN_TTL", defaultRunTTL.String()))
	if err != nil {
		return Config{}, err
	}
	origins, err := parseAllowedOrigins(envOrDefault("RECOMMENDATION_ALLOWED_ORIGINS", ""))
	if err != nil {
		return Config{}, err
	}
	eventsConfig, err := loadEventsConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress: net.JoinHostPort(
			envOrDefault("RECOMMENDATION_HTTP_HOST", defaultHTTPHost),
			httpPort,
		),
		GRPCAddress:        net.JoinHostPort(envOrDefault("RECOMMENDATION_GRPC_HOST", defaultHTTPHost), recommendationGRPCPort),
		UserAddress:        net.JoinHostPort(envOrDefault("USER_GRPC_HOST", defaultGRPCHost), grpcPort),
		InteractionAddress: net.JoinHostPort(envOrDefault("INTERACTION_GRPC_HOST", defaultGRPCHost), interactionPort),
		HistoryAddress:     net.JoinHostPort(envOrDefault("HISTORY_GRPC_HOST", defaultGRPCHost), historyPort),
		CatalogAddress:     net.JoinHostPort(envOrDefault("CATALOG_GRPC_HOST", defaultGRPCHost), catalogPort),
		ContentAddress:     net.JoinHostPort(envOrDefault("CONTENT_GRPC_HOST", defaultGRPCHost), contentPort),
		RunTTL:             runTTL,
		Database:           databaseConfig,
		Identity: identity.Config{
			JWKSURL:         envOrDefault("RECOMMENDATION_AUTH_JWKS_URL", defaultJWKSURL),
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
	topic := strings.TrimSpace(envOrDefault("KAFKA_HISTORY_TOPIC", defaultHistoryTopic))
	clearedTopic := strings.TrimSpace(envOrDefault(
		"KAFKA_HISTORY_CLEARED_TOPIC", defaultHistoryClearedTopic,
	))
	group := strings.TrimSpace(envOrDefault("KAFKA_RECOMMENDATION_GROUP", defaultConsumerGroup))
	if topic == "" || clearedTopic == "" || group == "" {
		return EventsConfig{}, fmt.Errorf("Kafka topics and consumer group must not be blank")
	}
	return EventsConfig{
		Brokers: brokers, HistoryTopic: topic,
		HistoryClearedTopic: clearedTopic, ConsumerGroup: group,
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
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid RECOMMENDATION_ALLOWED_ORIGINS entry %q", origin)
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		result = append(result, origin)
	}
	return result, nil
}

func parsePositiveDuration(key, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid %s %q: expected a positive duration", key, value)
	}
	return duration, nil
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
