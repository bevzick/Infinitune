package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const PreferenceChangedV1 = "interaction.preference.changed.v1"

type interactionEnvelope struct {
	ID         uuid.UUID             `json:"id"`
	Type       string                `json:"type"`
	Source     string                `json:"source"`
	OccurredAt time.Time             `json:"occurred_at"`
	Data       preferenceChangedData `json:"data"`
}

type preferenceChangedData struct {
	UserID     uuid.UUID `json:"user_id"`
	TargetType string    `json:"target_type"`
	TargetID   uuid.UUID `json:"target_id"`
	Action     string    `json:"action"`
}

type InteractionHandler struct{ runs RunInvalidator }

func NewInteractionHandler(runs RunInvalidator) *InteractionHandler {
	return &InteractionHandler{runs: runs}
}

func (handler *InteractionHandler) Handle(ctx context.Context, payload []byte) error {
	var event interactionEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("%w: decode interaction event: %v", ErrInvalidEvent, err)
	}
	if event.ID == uuid.Nil || event.Type != PreferenceChangedV1 ||
		event.Source != "interaction-service" || event.OccurredAt.IsZero() ||
		event.Data.UserID == uuid.Nil || event.Data.TargetID == uuid.Nil ||
		!validPreferenceChange(event.Data.TargetType, event.Data.Action) {
		return fmt.Errorf("%w: incomplete interaction preference event", ErrInvalidEvent)
	}
	if _, err := handler.runs.InvalidateBefore(ctx, event.Data.UserID, event.OccurredAt); err != nil {
		return fmt.Errorf("invalidate runs after preference change: %w", err)
	}
	return nil
}

func validPreferenceChange(targetType, action string) bool {
	switch targetType {
	case "track", "album":
		return action == "liked" || action == "unliked"
	case "artist":
		return action == "followed" || action == "unfollowed"
	default:
		return false
	}
}
