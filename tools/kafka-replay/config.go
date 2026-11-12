package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBrokers         = "127.0.0.1:9092"
	defaultDeadLetterTopic = "platform.events.dead-letter.v1"
)

type config struct {
	brokers         []string
	deadLetterTopic string
	consumerGroup   string
	originalTopics  map[string]struct{}
	execute         bool
	maxEvents       int
	idleTimeout     time.Duration
	deliveryTimeout time.Duration
}

type environment func(string) (string, bool)

func parseConfig(arguments []string, lookup environment, output io.Writer) (config, error) {
	flags := flag.NewFlagSet("kafka-replay", flag.ContinueOnError)
	flags.SetOutput(output)
	brokers := flags.String("brokers", envOrDefault(lookup, "KAFKA_BROKERS", defaultBrokers), "comma-separated Kafka brokers")
	deadLetterTopic := flags.String("dead-letter-topic", envOrDefault(lookup, "KAFKA_DEAD_LETTER_TOPIC", defaultDeadLetterTopic), "dead-letter topic to consume")
	consumerGroup := flags.String("consumer-group", envOrDefault(lookup, "KAFKA_REPLAY_GROUP", ""), "required consumer group for --execute")
	originalTopics := flags.String("original-topics", "", "required comma-separated allowlist of replay destinations")
	execute := flags.Bool("execute", false, "publish and commit records; default is preview only")
	maxEvents := flags.Int("max-events", envIntOrDefault(lookup, "KAFKA_REPLAY_MAX_EVENTS", 100), "maximum matching records to inspect")
	idleTimeout := flags.Duration("idle-timeout", 5*time.Second, "stop after this period without a record")
	deliveryTimeout := flags.Duration("delivery-timeout", 15*time.Second, "Kafka produce timeout")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	parsedBrokers, err := parseList(*brokers, "brokers")
	if err != nil {
		return config{}, err
	}
	parsedTopics, err := parseList(*originalTopics, "original-topics")
	if err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*deadLetterTopic) == "" {
		return config{}, errors.New("dead-letter-topic must not be blank")
	}
	if *maxEvents < 1 || *maxEvents > 10_000 {
		return config{}, errors.New("max-events must be between 1 and 10000")
	}
	if *idleTimeout <= 0 || *deliveryTimeout <= 0 {
		return config{}, errors.New("idle-timeout and delivery-timeout must be positive")
	}
	if *execute && strings.TrimSpace(*consumerGroup) == "" {
		return config{}, errors.New("consumer-group is required with --execute")
	}
	topics := make(map[string]struct{}, len(parsedTopics))
	for _, topic := range parsedTopics {
		if topic == strings.TrimSpace(*deadLetterTopic) {
			return config{}, errors.New("dead-letter topic cannot be a replay destination")
		}
		topics[topic] = struct{}{}
	}
	return config{
		brokers: parsedBrokers, deadLetterTopic: strings.TrimSpace(*deadLetterTopic),
		consumerGroup: strings.TrimSpace(*consumerGroup), originalTopics: topics,
		execute: *execute, maxEvents: *maxEvents,
		idleTimeout: *idleTimeout, deliveryTimeout: *deliveryTimeout,
	}, nil
}

func parseList(value, name string) ([]string, error) {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("%s must contain non-blank comma-separated values", name)
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return result, nil
}

func envOrDefault(lookup environment, name, fallback string) string {
	if value, exists := lookup(name); exists {
		return value
	}
	return fallback
}

func envIntOrDefault(lookup environment, name string, fallback int) int {
	value, exists := lookup(name)
	if !exists || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return parsed
}
