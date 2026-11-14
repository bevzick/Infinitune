package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestInteractionHandlerInvalidatesUserRuns(t *testing.T) {
	userID := uuid.New()
	invalidator := &invalidatorStub{}
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"interaction.preference.changed.v1","source":"interaction-service","occurred_at":"2026-11-12T12:00:00Z","data":{"user_id":"` + userID.String() + `","target_type":"track","target_id":"` + uuid.NewString() + `","action":"liked"}}`)
	if err := NewInteractionHandler(invalidator).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if invalidator.userID != userID || !invalidator.before.Equal(time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("invalidator = %+v", invalidator)
	}
}

func TestInteractionHandlerRejectsInvalidAction(t *testing.T) {
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"interaction.preference.changed.v1","source":"interaction-service","occurred_at":"2026-11-12T12:00:00Z","data":{"user_id":"` + uuid.NewString() + `","target_type":"artist","target_id":"` + uuid.NewString() + `","action":"liked"}}`)
	err := NewInteractionHandler(&invalidatorStub{}).Handle(context.Background(), payload)
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Handle() error = %v, want %v", err, ErrInvalidEvent)
	}
}
