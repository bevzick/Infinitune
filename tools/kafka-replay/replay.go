package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

var deadLetterHeaderSuffix = []string{
	"dlq_original_topic",
	"dlq_original_partition",
	"dlq_original_offset",
	"dlq_consumer_group",
	"dlq_error",
	"dlq_failed_at",
}

type replaySummary struct {
	inspected int
	replayed  int
}

func replay(ctx context.Context, configuration config, logger *slog.Logger) (replaySummary, error) {
	options := []kgo.Opt{
		kgo.SeedBrokers(configuration.brokers...),
		kgo.ConsumeTopics(configuration.deadLetterTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(configuration.deliveryTimeout),
		kgo.RecordPartitioner(kgo.ManualPartitioner()),
	}
	if configuration.execute {
		options = append(options, kgo.ConsumerGroup(configuration.consumerGroup), kgo.DisableAutoCommit())
	}
	client, err := kgo.NewClient(options...)
	if err != nil {
		return replaySummary{}, fmt.Errorf("create Kafka client: %w", err)
	}
	defer client.Close()

	summary := replaySummary{}
	for summary.inspected < configuration.maxEvents {
		pollContext, cancel := context.WithTimeout(ctx, configuration.idleTimeout)
		fetches := client.PollRecords(pollContext, 1)
		pollError := pollContext.Err()
		cancel()
		if ctx.Err() != nil {
			return summary, ctx.Err()
		}
		if errors.Is(pollError, context.DeadlineExceeded) && fetches.NumRecords() == 0 {
			return summary, nil
		}
		if fetchErrors := fetches.Errors(); len(fetchErrors) > 0 {
			return summary, fmt.Errorf("consume dead-letter record: %w", fetchErrors[0].Err)
		}
		var source *kgo.Record
		fetches.EachRecord(func(record *kgo.Record) { source = record })
		if source == nil {
			continue
		}
		replayed, err := replayRecord(source, configuration.deadLetterTopic)
		if err != nil {
			return summary, fmt.Errorf("dead-letter offset %d: %w", source.Offset, err)
		}
		if _, allowed := configuration.originalTopics[replayed.Topic]; !allowed {
			return summary, fmt.Errorf("dead-letter offset %d targets non-allowlisted topic %q", source.Offset, replayed.Topic)
		}
		summary.inspected++
		logger.Info("inspected dead-letter event",
			"execute", configuration.execute, "dlq_partition", source.Partition,
			"dlq_offset", source.Offset, "original_topic", replayed.Topic,
			"original_partition", replayed.Partition,
		)
		if !configuration.execute {
			continue
		}
		if err = client.ProduceSync(ctx, replayed).FirstErr(); err != nil {
			return summary, fmt.Errorf("publish dead-letter offset %d to %s: %w", source.Offset, replayed.Topic, err)
		}
		if err = client.CommitRecords(ctx, source); err != nil {
			return summary, fmt.Errorf("commit dead-letter offset %d: %w", source.Offset, err)
		}
		summary.replayed++
	}
	return summary, nil
}

func replayRecord(source *kgo.Record, deadLetterTopic string) (*kgo.Record, error) {
	if source.Topic != deadLetterTopic {
		return nil, fmt.Errorf("record topic is %q, expected %q", source.Topic, deadLetterTopic)
	}
	if len(source.Headers) < len(deadLetterHeaderSuffix) {
		return nil, errors.New("dead-letter metadata headers are missing")
	}
	metadataStart := len(source.Headers) - len(deadLetterHeaderSuffix)
	metadata := source.Headers[metadataStart:]
	for index, name := range deadLetterHeaderSuffix {
		if metadata[index].Key != name {
			return nil, fmt.Errorf("dead-letter metadata suffix is invalid at %q", name)
		}
	}
	originalTopic := string(metadata[0].Value)
	if originalTopic == "" || originalTopic == deadLetterTopic {
		return nil, fmt.Errorf("invalid original topic %q", originalTopic)
	}
	originalPartition, err := strconv.ParseInt(string(metadata[1].Value), 10, 32)
	if err != nil || originalPartition < 0 {
		return nil, fmt.Errorf("invalid original partition %q", metadata[1].Value)
	}
	return &kgo.Record{
		Topic: originalTopic, Partition: int32(originalPartition),
		Key: append([]byte(nil), source.Key...), Value: append([]byte(nil), source.Value...),
		Headers:   cloneHeaders(source.Headers[:metadataStart]),
		Timestamp: time.Now().UTC(),
	}, nil
}

func cloneHeaders(headers []kgo.RecordHeader) []kgo.RecordHeader {
	result := make([]kgo.RecordHeader, len(headers))
	for index, header := range headers {
		result[index] = kgo.RecordHeader{Key: header.Key, Value: append([]byte(nil), header.Value...)}
	}
	return result
}
