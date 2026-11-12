//go:build integration

package kafka

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/events"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestConsumerRoutesInvalidEventToDeadLetterTopic(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	brokers := strings.Split(envOrDefault("KAFKA_BROKERS", "kafka:9092"), ",")
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	sourceTopic := "integration.invalid." + suffix
	deadLetterTopic := "integration.dlq." + suffix
	groupID := "integration-content-" + suffix
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	admin, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	createConsumerIntegrationTopics(t, ctx, admin, sourceTopic, deadLetterTopic)
	t.Cleanup(func() {
		deleteConsumerIntegrationTopics(admin, sourceTopic, deadLetterTopic)
		admin.Close()
	})

	consumer, err := NewConsumer(
		brokers, sourceTopic, groupID, deadLetterTopic, invalidIntegrationHandler{},
		slog.New(slog.NewTextHandler(io.Discard, nil)), events.NewMetrics("content-integration"),
	)
	if err != nil {
		t.Fatal(err)
	}
	consumerContext, stopConsumer := context.WithCancel(ctx)
	consumerStopped := make(chan struct{})
	go func() {
		consumer.Run(consumerContext)
		close(consumerStopped)
	}()
	defer func() {
		stopConsumer()
		<-consumerStopped
		consumer.Close()
	}()

	source := &kgo.Record{
		Topic: sourceTopic, Key: []byte("asset-1"), Value: []byte("{invalid"),
		Headers: []kgo.RecordHeader{{Key: "event_type", Value: []byte("media.asset.ready.v1")}},
	}
	if err = admin.ProduceSync(ctx, source).FirstErr(); err != nil {
		t.Fatalf("publish invalid event: %v", err)
	}
	deadLetter := consumeDeadLetterIntegrationRecord(t, ctx, brokers, deadLetterTopic)
	if string(deadLetter.Key) != "asset-1" || string(deadLetter.Value) != "{invalid" {
		t.Fatalf("dead-letter key/value = %q/%q", deadLetter.Key, deadLetter.Value)
	}
	headers := make(map[string]string, len(deadLetter.Headers))
	for _, header := range deadLetter.Headers {
		headers[header.Key] = string(header.Value)
	}
	if headers["event_type"] != "media.asset.ready.v1" ||
		headers["dlq_original_topic"] != sourceTopic ||
		headers["dlq_consumer_group"] != groupID ||
		headers["dlq_error"] == "" {
		t.Fatalf("dead-letter headers = %v", headers)
	}
}

type invalidIntegrationHandler struct{}

func (invalidIntegrationHandler) Handle(context.Context, []byte) error {
	return fmt.Errorf("%w: integration payload", events.ErrInvalidEvent)
}

func createConsumerIntegrationTopics(t *testing.T, ctx context.Context, client *kgo.Client, topics ...string) {
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

func deleteConsumerIntegrationTopics(client *kgo.Client, topics ...string) {
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

func consumeDeadLetterIntegrationRecord(t *testing.T, ctx context.Context, brokers []string, topic string) *kgo.Record {
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
		t.Fatalf("consume dead-letter record: %v", fetchErrors[0].Err)
	}
	var record *kgo.Record
	fetches.EachRecord(func(current *kgo.Record) { record = current })
	if record == nil {
		t.Fatal("dead-letter record was not received")
	}
	return record
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
