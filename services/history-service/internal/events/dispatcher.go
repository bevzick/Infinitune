package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	claimLease       = 30 * time.Second
	maximumRetryWait = 5 * time.Minute
)

type Dispatcher struct {
	outbox    Outbox
	publisher Publisher
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
	metrics   *Metrics
	now       func() time.Time
}

func NewDispatcher(
	outbox Outbox,
	publisher Publisher,
	logger *slog.Logger,
	interval time.Duration,
	batchSize int,
	metrics *Metrics,
) *Dispatcher {
	return &Dispatcher{
		outbox: outbox, publisher: publisher, logger: logger,
		interval: interval, batchSize: batchSize, metrics: metrics, now: time.Now,
	}
}

func (dispatcher *Dispatcher) Run(ctx context.Context) {
	dispatcher.dispatch(ctx)
	ticker := time.NewTicker(dispatcher.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatcher.dispatch(ctx)
		}
	}
}

func (dispatcher *Dispatcher) dispatch(ctx context.Context) {
	now := dispatcher.now().UTC()
	messages, err := dispatcher.outbox.Claim(ctx, dispatcher.batchSize, now.Add(claimLease))
	if err != nil {
		dispatcher.metrics.OutboxOperationFailed()
		dispatcher.logger.Error("failed to claim outbox events", "error", err)
		return
	}
	dispatcher.metrics.OutboxClaimed(len(messages))
	for _, message := range messages {
		if err = dispatcher.publisher.Publish(ctx, message); err != nil {
			dispatcher.metrics.OutboxPublishFailed()
			retryAt := now.Add(retryDelay(message.Attempts))
			if markErr := dispatcher.outbox.MarkFailed(ctx, message.ID, err.Error(), retryAt); markErr != nil {
				dispatcher.metrics.OutboxOperationFailed()
				dispatcher.logger.Error("failed to release outbox event", "event_id", message.ID, "error", markErr)
			}
			dispatcher.logger.Warn("failed to publish outbox event", "event_id", message.ID, "event_type", message.Type, "error", err)
			continue
		}
		publishedAt := dispatcher.now().UTC()
		if err = dispatcher.outbox.MarkPublished(ctx, message.ID, publishedAt); err != nil {
			dispatcher.metrics.OutboxOperationFailed()
			dispatcher.logger.Error("failed to mark outbox event published", "event_id", message.ID, "error", err)
			continue
		}
		dispatcher.metrics.OutboxPublished(publishedAt)
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := min(attempt-1, 8)
	delay := time.Second * time.Duration(1<<shift)
	if delay > maximumRetryWait {
		return maximumRetryWait
	}
	return delay
}

func ValidateDispatcherConfig(interval time.Duration, batchSize int) error {
	if interval <= 0 {
		return fmt.Errorf("outbox interval must be positive")
	}
	if batchSize < 1 || batchSize > 1000 {
		return fmt.Errorf("outbox batch size must be between 1 and 1000")
	}
	return nil
}
