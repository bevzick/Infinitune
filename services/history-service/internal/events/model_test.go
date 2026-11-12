package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
)

func TestNewListeningRecordedBuildsVersionedEvent(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	event := history.Event{
		ID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(),
		StartedAt: finishedAt.Add(-time.Minute), FinishedAt: &finishedAt,
		ListenedMS: 60000, Completed: true, ContextType: "direct", DeviceType: "web",
	}
	message, err := NewListeningRecorded(event, "history-events")
	if err != nil {
		t.Fatalf("NewListeningRecorded() error = %v", err)
	}
	if message.ID != event.ID || message.Type != ListeningRecordedV1 ||
		message.Topic != "history-events" || message.Key != event.UserID.String() ||
		!message.OccurredAt.Equal(finishedAt) {
		t.Fatalf("message = %+v", message)
	}
	var envelope Envelope[ListeningRecordedData]
	if err = json.Unmarshal(message.Payload, &envelope); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if envelope.ID != event.ID || envelope.Type != ListeningRecordedV1 ||
		envelope.Data.UserID != event.UserID || envelope.Data.TrackID != event.TrackID ||
		!envelope.Data.Completed {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestNewListeningClearedBuildsDeletionWatermark(t *testing.T) {
	t.Parallel()

	eventID, userID := uuid.New(), uuid.New()
	clearedAt := time.Date(2026, time.November, 12, 13, 0, 0, 0, time.UTC)
	before := clearedAt.Add(-24 * time.Hour)
	message, err := NewListeningCleared(
		eventID, userID, &before, 7, clearedAt, "history-cleared",
	)
	if err != nil {
		t.Fatalf("NewListeningCleared() error = %v", err)
	}
	if message.ID != eventID || message.Type != ListeningClearedV1 ||
		message.Topic != "history-cleared" || message.Key != userID.String() {
		t.Fatalf("message = %+v", message)
	}
	var envelope Envelope[ListeningClearedData]
	if err = json.Unmarshal(message.Payload, &envelope); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if envelope.Data.UserID != userID || envelope.Data.DeletedCount != 7 ||
		envelope.Data.Before == nil || !envelope.Data.Before.Equal(before) {
		t.Fatalf("envelope = %+v", envelope)
	}
}
