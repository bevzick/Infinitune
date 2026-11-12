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
