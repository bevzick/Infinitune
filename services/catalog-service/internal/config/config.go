package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/database"
)

const (
	defaultHTTPHost             = "0.0.0.0"
	defaultHTTPPort             = "8080"
	defaultGRPCHost             = "0.0.0.0"
	defaultGRPCPort             = "9090"
	defaultPostgresHost         = "127.0.0.1"
	defaultPostgresPort         = "5432"
	defaultKafkaBrokers         = "127.0.0.1:9092"
	defaultPublishedTopic       = "catalog.release.published.v1"
	defaultArchivedTopic        = "catalog.release.archived.v1"
	defaultContentApprovedTopic = "content.release.approved.v1"
	defaultContentConsumerGroup = "catalog-content-v1"
	defaultDeadLetterTopic      = "platform.events.dead-letter.v1"
	defaultOutboxPoll           = time.Second
	defaultOutboxBatch          = 100
)

type EventsConfig struct {
	Brokers              []string
	PublishedTopic       string
	ArchivedTopic        string
	ContentApprovedTopic string
	ContentConsumerGroup string
	DeadLetterTopic      string
	OutboxInterval       time.Duration
	OutboxBatch          int
}

type Config struct {
	HTTPAddress string
	GRPCAddress string
	Database    database.Config
	Origins     []string
	Events      EventsConfig
}

func Load() (Config, error) {
	port := envOrDefault("CATALOG_HTTP_PORT", defaultHTTPPort)
	if _, err := parsePort("CATALOG_HTTP_PORT", port); err != nil {
		return Config{}, err
	}
	grpcPort := envOrDefault("CATALOG_GRPC_PORT", defaultGRPCPort)
	if _, err := parsePort("CATALOG_GRPC_PORT", grpcPort); err != nil {
		return Config{}, err
	}

	databaseConfig, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	origins, err := parseAllowedOrigins(envOrDefault("CATALOG_ALLOWED_ORIGINS", ""))
	if err != nil {
		return Config{}, err
	}

	eventsConfig, err := loadEventsConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		HTTPAddress: net.JoinHostPort(envOrDefault("CATALOG_HTTP_HOST", defaultHTTPHost), port),
		GRPCAddress: net.JoinHostPort(envOrDefault("CATALOG_GRPC_HOST", defaultGRPCHost), grpcPort),
		Database:    databaseConfig,
		Origins:     origins,
		Events:      eventsConfig,
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
	publishedTopic := strings.TrimSpace(envOrDefault(
		"KAFKA_CATALOG_PUBLISHED_TOPIC", defaultPublishedTopic,
	))
	archivedTopic := strings.TrimSpace(envOrDefault(
		"KAFKA_CATALOG_ARCHIVED_TOPIC", defaultArchivedTopic,
	))
	approvedTopic := strings.TrimSpace(envOrDefault(
		"KAFKA_CONTENT_RELEASE_APPROVED_TOPIC", defaultContentApprovedTopic,
	))
	contentGroup := strings.TrimSpace(envOrDefault(
		"KAFKA_CATALOG_CONTENT_GROUP", defaultContentConsumerGroup,
	))
	deadLetterTopic := strings.TrimSpace(envOrDefault(
		"KAFKA_DEAD_LETTER_TOPIC", defaultDeadLetterTopic,
	))
	if publishedTopic == "" || archivedTopic == "" || approvedTopic == "" || contentGroup == "" || deadLetterTopic == "" {
		return EventsConfig{}, fmt.Errorf("Kafka Catalog topics must not be blank")
	}
	interval := defaultOutboxPoll
	if raw := os.Getenv("CATALOG_OUTBOX_POLL_INTERVAL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return EventsConfig{}, fmt.Errorf("invalid CATALOG_OUTBOX_POLL_INTERVAL %q", raw)
		}
		interval = parsed
	}
	batch := defaultOutboxBatch
	if raw := os.Getenv("CATALOG_OUTBOX_BATCH_SIZE"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000 {
			return EventsConfig{}, fmt.Errorf("invalid CATALOG_OUTBOX_BATCH_SIZE %q: expected 1..1000", raw)
		}
		batch = parsed
	}
	return EventsConfig{
		Brokers: brokers, PublishedTopic: publishedTopic, ArchivedTopic: archivedTopic,
		ContentApprovedTopic: approvedTopic, ContentConsumerGroup: contentGroup,
		DeadLetterTopic: deadLetterTopic,
		OutboxInterval:  interval, OutboxBatch: batch,
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
			return nil, fmt.Errorf("invalid CATALOG_ALLOWED_ORIGINS entry %q", origin)
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
	name, err := requiredEnv("POSTGRES_CATALOG_DB")
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
