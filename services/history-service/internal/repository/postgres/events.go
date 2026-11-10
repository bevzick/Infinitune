package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct{ pool *pgxpool.Pool }

func NewEventRepository(pool *pgxpool.Pool) *EventRepository { return &EventRepository{pool: pool} }

func (repository *EventRepository) Record(ctx context.Context, event history.Event) (history.Event, error) {
	row := repository.pool.QueryRow(ctx, `
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
	err := row.Scan(&stored.ID, &stored.UserID, &stored.TrackID, &stored.StartedAt, &stored.FinishedAt,
		&stored.ListenedMS, &stored.Completed, &stored.ContextType, &stored.ContextID, &stored.DeviceType)
	if errors.Is(err, pgx.ErrNoRows) {
		return history.Event{}, history.ErrEventConflict
	}
	if err != nil {
		return history.Event{}, fmt.Errorf("record listening event: %w", err)
	}
	return stored, nil
}
