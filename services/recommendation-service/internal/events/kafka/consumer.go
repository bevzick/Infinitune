package kafka

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/events"
	"github.com/twmb/franz-go/pkg/kgo"
)

const retryInterval = time.Second

type Handler interface {
	Handle(context.Context, []byte) error
}

type Consumer struct {
	client  *kgo.Client
	handler Handler
	logger  *slog.Logger
}

func NewConsumer(
	brokers []string,
	topic string,
	groupID string,
	handler Handler,
	logger *slog.Logger,
) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(groupID),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client, handler: handler, logger: logger}, nil
}

func (consumer *Consumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		fetches := consumer.client.PollRecords(ctx, 1)
		if ctx.Err() != nil {
			return
		}
		for _, fetchErr := range fetches.Errors() {
			consumer.logger.Error(
				"failed to fetch Kafka event",
				"topic", fetchErr.Topic, "partition", fetchErr.Partition, "error", fetchErr.Err,
			)
		}
		fetches.EachRecord(func(record *kgo.Record) {
			consumer.process(ctx, record)
		})
	}
}

func (consumer *Consumer) process(ctx context.Context, record *kgo.Record) {
	for ctx.Err() == nil {
		err := consumer.handler.Handle(ctx, record.Value)
		if err == nil {
			if err = consumer.client.CommitRecords(ctx, record); err != nil {
				consumer.logger.Error("failed to commit Kafka event", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			}
			return
		}
		if errors.Is(err, events.ErrInvalidEvent) {
			consumer.logger.Error("discarding invalid Kafka event", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			if commitErr := consumer.client.CommitRecords(ctx, record); commitErr != nil {
				consumer.logger.Error("failed to commit invalid Kafka event", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", commitErr)
			}
			return
		}
		consumer.logger.Warn("failed to handle Kafka event", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (consumer *Consumer) Close() {
	consumer.client.Close()
}
