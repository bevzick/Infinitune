package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const ListeningRecordedV1 = "history.listening.recorded.v1"

var ErrInvalidEvent = errors.New("invalid domain event")

type historyEnvelope struct {
	ID         uuid.UUID           `json:"id"`
	Type       string              `json:"type"`
	Source     string              `json:"source"`
	OccurredAt time.Time           `json:"occurred_at"`
	Data       listeningRecordData `json:"data"`
}

type listeningRecordData struct {
	UserID uuid.UUID `json:"user_id"`
}

type RunInvalidator interface {
	InvalidateBefore(context.Context, uuid.UUID, time.Time) (int64, error)
}

type HistoryHandler struct {
	runs RunInvalidator
}

func NewHistoryHandler(runs RunInvalidator) *HistoryHandler {
	return &HistoryHandler{runs: runs}
}

func (handler *HistoryHandler) Handle(ctx context.Context, payload []byte) error {
	var event historyEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("%w: decode history event: %v", ErrInvalidEvent, err)
	}
	if event.ID == uuid.Nil || event.Type != ListeningRecordedV1 ||
		event.Source != "history-service" || event.OccurredAt.IsZero() ||
		event.Data.UserID == uuid.Nil {
		return fmt.Errorf("%w: incomplete history event", ErrInvalidEvent)
	}
	if _, err := handler.runs.InvalidateBefore(ctx, event.Data.UserID, event.OccurredAt); err != nil {
		return fmt.Errorf("invalidate recommendation runs: %w", err)
	}
	return nil
}
