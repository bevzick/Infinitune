//go:build integration

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestReplayPublishesAndCommitsDeadLetterRecord(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	brokers := strings.Split(envOrDefault(os.LookupEnv, "KAFKA_BROKERS", defaultBrokers), ",")
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	deadLetterTopic := "integration.dlq." + suffix
	originalTopic := "integration.original." + suffix
	consumerGroup := "integration-replay-" + suffix
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	admin, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	createIntegrationTopics(t, ctx, admin, deadLetterTopic, originalTopic)
	t.Cleanup(func() {
		deleteIntegrationTopics(admin, deadLetterTopic, originalTopic)
		admin.Close()
	})

	source := integrationDeadLetterRecord(deadLetterTopic, originalTopic)
	if err = admin.ProduceSync(ctx, source).FirstErr(); err != nil {
		t.Fatalf("publish dead-letter record: %v", err)
	}
	configuration := config{
		brokers: brokers, deadLetterTopic: deadLetterTopic, consumerGroup: consumerGroup,
		originalTopics: map[string]struct{}{originalTopic: {}}, execute: true,
		maxEvents: 1, idleTimeout: 2 * time.Second, deliveryTimeout: 10 * time.Second,
	}
	summary, err := replay(ctx, configuration, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if summary.inspected != 1 || summary.replayed != 1 {
		t.Fatalf("summary = %+v", summary)
	}

	replayed := consumeIntegrationRecord(t, ctx, brokers, originalTopic)
	if replayed.Partition != 0 || string(replayed.Key) != "user-1" || string(replayed.Value) != "payload" || len(replayed.Headers) != 1 || replayed.Headers[0].Key != "event_id" {
		t.Fatalf("replayed record = %+v", replayed)
	}
	configuration.idleTimeout = time.Second
	summary, err = replay(ctx, configuration, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if summary.inspected != 0 || summary.replayed != 0 {
		t.Fatalf("committed replay summary = %+v", summary)
	}
}

func createIntegrationTopics(t *testing.T, ctx context.Context, client *kgo.Client, topics ...string) {
	t.Helper()
	request := kmsg.NewPtrCreateTopicsRequest()
	for _, topic := range topics {
		request.Topics = append(request.Topics, kmsg.CreateTopicsRequestTopic{
			Topic: topic, NumPartitions: 1, ReplicationFactor: 1,
		})
	}
	response, err := request.RequestWith(ctx, client)
	if err != nil {
		t.Fatalf("create topics: %v", err)
	}
	for _, result := range response.Topics {
		if topicErr := kerr.ErrorForCode(result.ErrorCode); topicErr != nil {
			t.Fatalf("create topic %s: %v", result.Topic, topicErr)
		}
	}
}

func deleteIntegrationTopics(client *kgo.Client, topics ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := kmsg.NewPtrDeleteTopicsRequest()
	request.TopicNames = append(request.TopicNames, topics...)
	for _, topic := range topics {
		topicName := topic
		request.Topics = append(request.Topics, kmsg.DeleteTopicsRequestTopic{Topic: &topicName})
	}
	_, _ = request.RequestWith(ctx, client)
}

func integrationDeadLetterRecord(deadLetterTopic, originalTopic string) *kgo.Record {
	return &kgo.Record{
		Topic: deadLetterTopic, Key: []byte("user-1"), Value: []byte("payload"),
		Headers: []kgo.RecordHeader{
			{Key: "event_id", Value: []byte("event-1")},
			{Key: "dlq_original_topic", Value: []byte(originalTopic)},
			{Key: "dlq_original_partition", Value: []byte("0")},
			{Key: "dlq_original_offset", Value: []byte("7")},
			{Key: "dlq_consumer_group", Value: []byte("integration-source-v1")},
			{Key: "dlq_error", Value: []byte("invalid event")},
			{Key: "dlq_failed_at", Value: []byte(time.Now().UTC().Format(time.RFC3339Nano))},
		},
	}
}

func consumeIntegrationRecord(t *testing.T, ctx context.Context, brokers []string, topic string) *kgo.Record {
	t.Helper()
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{topic: {0: kgo.NewOffset().AtStart()}}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	fetches := client.PollRecords(ctx, 1)
	if fetchErrors := fetches.Errors(); len(fetchErrors) > 0 {
		t.Fatalf("consume replayed record: %v", fetchErrors[0].Err)
	}
	var record *kgo.Record
	fetches.EachRecord(func(current *kgo.Record) { record = current })
	if record == nil {
		t.Fatal("replayed record was not received")
	}
	return record
}
