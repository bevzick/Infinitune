package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewPreferenceChangedBuildsVersionedEvent(t *testing.T) {
	t.Parallel()
	userID, trackID := uuid.New(), uuid.New()
	occurredAt := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	message, err := NewPreferenceChanged(userID, TargetTrack, trackID, ActionLiked, occurredAt, "preferences")
	if err != nil {
		t.Fatalf("NewPreferenceChanged() error = %v", err)
	}
	if message.AggregateID != userID || message.Key != userID.String() || message.Type != PreferenceChangedV1 || message.Topic != "preferences" || !message.OccurredAt.Equal(occurredAt) {
		t.Fatalf("message = %+v", message)
	}
	var envelope Envelope[PreferenceChangedData]
	if err = json.Unmarshal(message.Payload, &envelope); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if envelope.Source != "interaction-service" || envelope.Data.UserID != userID || envelope.Data.TargetType != TargetTrack || envelope.Data.TargetID != trackID || envelope.Data.Action != ActionLiked {
		t.Fatalf("envelope = %+v", envelope)
	}
}
