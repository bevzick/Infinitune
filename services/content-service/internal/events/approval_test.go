package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/google/uuid"
)

func TestNewReleaseApprovedBuildsVersionedEvent(t *testing.T) {
	t.Parallel()
	creatorID := uuid.New()
	review := moderation.Review{
		ID: uuid.New(), ReleaseID: uuid.New(), ReviewerUserID: uuid.New(),
		ReviewedSubmissionAt: time.Date(2026, 11, 12, 11, 0, 0, 0, time.UTC),
		CreatedAt:            time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
	}
	message, err := NewReleaseApproved(review, creatorID, "content-approved")
	if err != nil {
		t.Fatalf("NewReleaseApproved() error = %v", err)
	}
	if message.ID != review.ID || message.AggregateID != review.ReleaseID ||
		message.Key != review.ReleaseID.String() || message.Type != ReleaseApprovedV1 ||
		message.Topic != "content-approved" || !message.OccurredAt.Equal(review.CreatedAt) {
		t.Fatalf("message = %+v", message)
	}
	var envelope Envelope[ReleaseApprovedData]
	if err = json.Unmarshal(message.Payload, &envelope); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if envelope.Source != "content-service" || envelope.Data.ReleaseID != review.ReleaseID ||
		envelope.Data.CreatorID != creatorID || envelope.Data.ReviewerUserID != review.ReviewerUserID ||
		!envelope.Data.ReviewedSubmissionAt.Equal(review.ReviewedSubmissionAt) {
		t.Fatalf("envelope = %+v", envelope)
	}
}
