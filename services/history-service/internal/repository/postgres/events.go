package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/events"
	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct {
	pool          *pgxpool.Pool
	recordedTopic string
	clearedTopic  string
}

func NewEventRepository(
	pool *pgxpool.Pool,
	recordedTopic string,
	clearedTopic string,
) *EventRepository {
	return &EventRepository{
		pool: pool, recordedTopic: recordedTopic, clearedTopic: clearedTopic,
	}
}

func (repository *EventRepository) Record(ctx context.Context, event history.Event) (history.Event, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return history.Event{}, fmt.Errorf("begin listening event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row := tx.QueryRow(ctx, `
		INSERT INTO history.listening_events (
			id, user_id, track_id, started_at, finished_at, listened_ms,
			completed, context_type, context_id, device_type
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET id = history.listening_events.id
		WHERE history.listening_events.user_id = EXCLUDED.user_id
		  AND history.listening_events.track_id = EXCLUDED.track_id
		  AND history.listening_events.started_at = EXCLUDED.started_at
		  AND history.listening_events.finished_at IS NOT DISTINCT FROM EXCLUDED.finished_at
		  AND history.listening_events.listened_ms = EXCLUDED.listened_ms
		  AND history.listening_events.completed = EXCLUDED.completed
		  AND history.listening_events.context_type = EXCLUDED.context_type
		  AND history.listening_events.context_id IS NOT DISTINCT FROM EXCLUDED.context_id
		  AND history.listening_events.device_type = EXCLUDED.device_type
		RETURNING id, user_id, track_id, started_at, finished_at, listened_ms,
		          completed, context_type, context_id, device_type
	`, event.ID, event.UserID, event.TrackID, event.StartedAt, event.FinishedAt, event.ListenedMS,
		event.Completed, event.ContextType, event.ContextID, event.DeviceType)
	var stored history.Event
	err = row.Scan(&stored.ID, &stored.UserID, &stored.TrackID, &stored.StartedAt, &stored.FinishedAt,
		&stored.ListenedMS, &stored.Completed, &stored.ContextType, &stored.ContextID, &stored.DeviceType)
	if errors.Is(err, pgx.ErrNoRows) {
		return history.Event{}, history.ErrEventConflict
	}
	if err != nil {
		return history.Event{}, fmt.Errorf("record listening event: %w", err)
	}
	message, err := events.NewListeningRecorded(stored, repository.recordedTopic)
	if err != nil {
		return history.Event{}, fmt.Errorf("encode listening event: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO history.event_outbox (
			id, event_type, topic, partition_key, payload, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO NOTHING
	`, message.ID, message.Type, message.Topic, message.Key, message.Payload, message.OccurredAt); err != nil {
		return history.Event{}, fmt.Errorf("enqueue listening event: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return history.Event{}, fmt.Errorf("commit listening event: %w", err)
	}
	return stored, nil
}

func (repository *EventRepository) List(ctx context.Context, userID uuid.UUID, filter history.ListFilter, limit int) ([]history.Event, error) {
	var cursorTime *time.Time
	var cursorID *uuid.UUID
	if filter.Cursor != nil {
		cursorTime = &filter.Cursor.StartedAt
		cursorID = &filter.Cursor.EventID
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id,user_id,track_id,started_at,finished_at,listened_ms,
		       completed,context_type,context_id,device_type
		FROM history.listening_events
		WHERE user_id=$1
		  AND ($2::uuid IS NULL OR track_id=$2)
		  AND ($3::timestamptz IS NULL OR started_at >= $3)
		  AND ($4::timestamptz IS NULL OR started_at < $4)
		  AND ($5::boolean IS NULL OR completed=$5)
		  AND ($6::text IS NULL OR context_type=$6)
		  AND ($7::text IS NULL OR device_type=$7)
		  AND ($8::timestamptz IS NULL OR (started_at,id) < ($8,$9))
		ORDER BY started_at DESC,id DESC
		LIMIT $10
	`, userID, filter.TrackID, filter.From, filter.To, filter.Completed, nullableString(filter.ContextType), nullableString(filter.DeviceType), cursorTime, cursorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list listening events: %w", err)
	}
	defer rows.Close()
	items := make([]history.Event, 0)
	for rows.Next() {
		var event history.Event
		if err = rows.Scan(&event.ID, &event.UserID, &event.TrackID, &event.StartedAt, &event.FinishedAt, &event.ListenedMS, &event.Completed, &event.ContextType, &event.ContextID, &event.DeviceType); err != nil {
			return nil, fmt.Errorf("scan listening event: %w", err)
		}
		items = append(items, event)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listening events: %w", err)
	}
	return items, nil
}

func (repository *EventRepository) Delete(
	ctx context.Context,
	userID uuid.UUID,
	before *time.Time,
	deletedAt time.Time,
) (int64, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin history deletion transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		DELETE FROM history.listening_events
		WHERE user_id=$1 AND ($2::timestamptz IS NULL OR started_at < $2)
	`, userID, before)
	if err != nil {
		return 0, fmt.Errorf("delete listening events: %w", err)
	}
	deleted := result.RowsAffected()
	if deleted > 0 {
		message, messageErr := events.NewListeningCleared(
			uuid.New(), userID, before, deleted, deletedAt, repository.clearedTopic,
		)
		if messageErr != nil {
			return 0, fmt.Errorf("encode history deletion event: %w", messageErr)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO history.event_outbox (
				id, event_type, topic, partition_key, payload, occurred_at
			) VALUES ($1,$2,$3,$4,$5,$6)
		`, message.ID, message.Type, message.Topic, message.Key,
			message.Payload, message.OccurredAt); err != nil {
			return 0, fmt.Errorf("enqueue history deletion event: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit history deletion: %w", err)
	}
	return deleted, nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
