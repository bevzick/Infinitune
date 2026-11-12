package events

import (
	"context"
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
	now       func() time.Time
}

func NewDispatcher(outbox Outbox, publisher Publisher, logger *slog.Logger, interval time.Duration, batchSize int) *Dispatcher {
	return &Dispatcher{outbox: outbox, publisher: publisher, logger: logger, interval: interval, batchSize: batchSize, now: time.Now}
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
		dispatcher.logger.Error("failed to claim media outbox events", "error", err)
		return
	}
	for _, message := range messages {
		if err = dispatcher.publisher.Publish(ctx, message); err != nil {
			retryAt := now.Add(retryDelay(message.Attempts))
			if markErr := dispatcher.outbox.MarkFailed(ctx, message.ID, err.Error(), retryAt); markErr != nil {
				dispatcher.logger.Error("failed to release media outbox event", "event_id", message.ID, "error", markErr)
			}
			continue
		}
		if err = dispatcher.outbox.MarkPublished(ctx, message.ID, dispatcher.now().UTC()); err != nil {
			dispatcher.logger.Error("failed to mark media event published", "event_id", message.ID, "error", err)
		}
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
