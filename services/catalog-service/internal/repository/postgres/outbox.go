package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/events"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository struct{ pool *pgxpool.Pool }

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (repository *OutboxRepository) Claim(
	ctx context.Context,
	limit int,
	claimedUntil time.Time,
) ([]events.Message, error) {
	rows, err := repository.pool.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM catalog.event_outbox
			WHERE published_at IS NULL
			  AND available_at <= now()
			  AND (claimed_until IS NULL OR claimed_until < now())
			ORDER BY occurred_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		), claimed AS (
			UPDATE catalog.event_outbox AS outbox
			SET claimed_until=$2, attempts=outbox.attempts+1
			FROM candidates
			WHERE outbox.id=candidates.id
			RETURNING outbox.id, outbox.aggregate_id, outbox.event_type, outbox.topic,
			          outbox.partition_key, outbox.payload, outbox.occurred_at, outbox.attempts
		)
		SELECT id, aggregate_id, event_type, topic, partition_key, payload, occurred_at, attempts
		FROM claimed
		ORDER BY occurred_at, id
	`, limit, claimedUntil)
	if err != nil {
		return nil, fmt.Errorf("claim catalog event outbox: %w", err)
	}
	defer rows.Close()
	messages := make([]events.Message, 0, limit)
	for rows.Next() {
		var message events.Message
		if err = rows.Scan(
			&message.ID, &message.AggregateID, &message.Type, &message.Topic,
			&message.Key, &message.Payload, &message.OccurredAt, &message.Attempts,
		); err != nil {
			return nil, fmt.Errorf("scan catalog event outbox: %w", err)
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate catalog event outbox: %w", err)
	}
	return messages, nil
}

func (repository *OutboxRepository) MarkPublished(
	ctx context.Context,
	id uuid.UUID,
	publishedAt time.Time,
) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE catalog.event_outbox
		SET published_at=$2, claimed_until=NULL, last_error=NULL
		WHERE id=$1 AND published_at IS NULL
	`, id, publishedAt)
	if err != nil {
		return fmt.Errorf("mark catalog outbox event published: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("mark catalog outbox event published: event %s is not pending", id)
	}
	return nil
}

func (repository *OutboxRepository) MarkFailed(
	ctx context.Context,
	id uuid.UUID,
	message string,
	retryAt time.Time,
) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE catalog.event_outbox
		SET available_at=$2, claimed_until=NULL, last_error=left($3, 2000)
		WHERE id=$1 AND published_at IS NULL
	`, id, retryAt, message)
	if err != nil {
		return fmt.Errorf("release catalog outbox event: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("release catalog outbox event: event %s is not pending", id)
	}
	return nil
}
