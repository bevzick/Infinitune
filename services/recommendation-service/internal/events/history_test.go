package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHistoryHandlerInvalidatesOlderRecommendationRuns(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	occurredAt := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	invalidator := &invalidatorStub{}
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"history.listening.recorded.v1",` +
		`"source":"history-service","occurred_at":"2026-11-12T12:00:00Z",` +
		`"data":{"user_id":"` + userID.String() + `"}}`)
	if err := NewHistoryHandler(invalidator).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if invalidator.userID != userID || !invalidator.before.Equal(occurredAt) {
		t.Fatalf("invalidation = %s before %v", invalidator.userID, invalidator.before)
	}
}

func TestHistoryHandlerRejectsMalformedEvent(t *testing.T) {
	t.Parallel()

	err := NewHistoryHandler(&invalidatorStub{}).Handle(context.Background(), []byte(`{"type":"unknown"}`))
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Handle() error = %v, want %v", err, ErrInvalidEvent)
	}
}

type invalidatorStub struct {
	userID uuid.UUID
	before time.Time
}

func (stub *invalidatorStub) InvalidateBefore(
	_ context.Context,
	userID uuid.UUID,
	before time.Time,
) (int64, error) {
	stub.userID = userID
	stub.before = before
	return 1, nil
}
