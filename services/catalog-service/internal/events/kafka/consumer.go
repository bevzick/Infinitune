package kafka

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/events"
	"github.com/twmb/franz-go/pkg/kgo"
)

const retryInterval = time.Second

type Handler interface {
	Handle(context.Context, []byte) error
}
type Consumer struct {
	client          *kgo.Client
	handler         Handler
	logger          *slog.Logger
	groupID         string
	deadLetterTopic string
	metrics         *events.Metrics
}

func NewConsumer(brokers []string, topic, groupID, deadLetterTopic string, handler Handler, logger *slog.Logger, metrics *events.Metrics) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic), kgo.ConsumerGroup(groupID),
		kgo.DisableAutoCommit(), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client, handler: handler, logger: logger, groupID: groupID, deadLetterTopic: deadLetterTopic, metrics: metrics}, nil
}

func (consumer *Consumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		fetches := consumer.client.PollRecords(ctx, 1)
		if ctx.Err() != nil {
			return
		}
		for _, fetchErr := range fetches.Errors() {
			consumer.metrics.ConsumerOperationFailed()
			consumer.logger.Error("failed to fetch Kafka event", "topic", fetchErr.Topic, "partition", fetchErr.Partition, "error", fetchErr.Err)
		}
		fetches.EachRecord(func(record *kgo.Record) { consumer.process(ctx, record) })
	}
}

func (consumer *Consumer) process(ctx context.Context, record *kgo.Record) {
	for ctx.Err() == nil {
		err := consumer.handler.Handle(ctx, record.Value)
		if err == nil {
			consumer.metrics.ConsumerProcessed(time.Now().UTC())
			if err = consumer.client.CommitRecords(ctx, record); err != nil {
				consumer.metrics.ConsumerOperationFailed()
				consumer.logger.Error("failed to commit Kafka event", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			}
			return
		}
		consumer.metrics.ConsumerHandlerFailed()
		if errors.Is(err, events.ErrInvalidEvent) {
			if deadLetterErr := consumer.publishDeadLetter(ctx, record, err); deadLetterErr != nil {
				consumer.metrics.ConsumerDeadLetterPublishFailed()
				consumer.logger.Warn("failed to publish invalid event to dead-letter topic", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", deadLetterErr)
				if !waitRetry(ctx) {
					return
				}
				continue
			}
			consumer.metrics.ConsumerDeadLettered()
			consumer.logger.Error("routed invalid Kafka event to dead-letter topic", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			if commitErr := consumer.client.CommitRecords(ctx, record); commitErr != nil {
				consumer.metrics.ConsumerOperationFailed()
				consumer.logger.Error("failed to commit invalid Kafka event", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", commitErr)
			}
			return
		}
		consumer.logger.Warn("failed to handle Kafka event", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
		if !waitRetry(ctx) {
			return
		}
	}
}

func (consumer *Consumer) publishDeadLetter(ctx context.Context, original *kgo.Record, failure error) error {
	return consumer.client.ProduceSync(ctx, deadLetterRecord(original, consumer.deadLetterTopic, consumer.groupID, failure, time.Now().UTC())).FirstErr()
}

func deadLetterRecord(original *kgo.Record, topic, groupID string, failure error, failedAt time.Time) *kgo.Record {
	headers := append([]kgo.RecordHeader(nil), original.Headers...)
	headers = append(headers,
		kgo.RecordHeader{Key: "dlq_original_topic", Value: []byte(original.Topic)},
		kgo.RecordHeader{Key: "dlq_original_partition", Value: []byte(strconv.FormatInt(int64(original.Partition), 10))},
		kgo.RecordHeader{Key: "dlq_original_offset", Value: []byte(strconv.FormatInt(original.Offset, 10))},
		kgo.RecordHeader{Key: "dlq_consumer_group", Value: []byte(groupID)},
		kgo.RecordHeader{Key: "dlq_error", Value: []byte(failure.Error())},
		kgo.RecordHeader{Key: "dlq_failed_at", Value: []byte(failedAt.Format(time.RFC3339Nano))},
	)
	return &kgo.Record{Topic: topic, Key: append([]byte(nil), original.Key...), Value: append([]byte(nil), original.Value...), Headers: headers}
}

func waitRetry(ctx context.Context) bool {
	timer := time.NewTimer(retryInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (consumer *Consumer) Close() { consumer.client.Close() }
