package main

import (
	"io"
	"testing"
	"time"
)

func TestParseConfigDefaultsToPreviewAndRequiresTopicAllowlist(t *testing.T) {
	configuration, err := parseConfig(
		[]string{"--original-topics", "history.listening.recorded.v1, catalog.release.published.v1"},
		func(string) (string, bool) { return "", false }, io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.execute || configuration.consumerGroup != "" || configuration.maxEvents != 100 || configuration.idleTimeout != 5*time.Second {
		t.Fatalf("configuration = %+v", configuration)
	}
	if len(configuration.originalTopics) != 2 {
		t.Fatalf("original topics = %v", configuration.originalTopics)
	}
}

func TestParseConfigRequiresConsumerGroupForExecution(t *testing.T) {
	_, err := parseConfig(
		[]string{"--original-topics", "history.listening.recorded.v1", "--execute"},
		func(string) (string, bool) { return "", false }, io.Discard,
	)
	if err == nil {
		t.Fatal("expected missing consumer group error")
	}
}

func TestParseConfigRejectsDeadLetterDestination(t *testing.T) {
	_, err := parseConfig(
		[]string{"--original-topics", defaultDeadLetterTopic},
		func(string) (string, bool) { return "", false }, io.Discard,
	)
	if err == nil {
		t.Fatal("expected dead-letter destination error")
	}
}
