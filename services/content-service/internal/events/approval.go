package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/google/uuid"
)

const ReleaseApprovedV1 = "content.release.approved.v1"

type Envelope[T any] struct {
	ID         uuid.UUID `json:"id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       T         `json:"data"`
}

type ReleaseApprovedData struct {
	ReleaseID            uuid.UUID `json:"release_id"`
	CreatorID            uuid.UUID `json:"creator_id"`
	ReviewerUserID       uuid.UUID `json:"reviewer_user_id"`
	ReviewedSubmissionAt time.Time `json:"reviewed_submission_at"`
}

type Message struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	Type        string
	Topic       string
	Key         string
	Payload     []byte
	OccurredAt  time.Time
	Attempts    int
}

func NewReleaseApproved(review moderation.Review, creatorID uuid.UUID, topic string) (Message, error) {
	payload, err := json.Marshal(Envelope[ReleaseApprovedData]{
		ID: review.ID, Type: ReleaseApprovedV1, Source: "content-service",
		OccurredAt: review.CreatedAt.UTC(),
		Data: ReleaseApprovedData{
			ReleaseID: review.ReleaseID, CreatorID: creatorID,
			ReviewerUserID:       review.ReviewerUserID,
			ReviewedSubmissionAt: review.ReviewedSubmissionAt.UTC(),
		},
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: review.ID, AggregateID: review.ReleaseID, Type: ReleaseApprovedV1,
		Topic: topic, Key: review.ReleaseID.String(), Payload: payload,
		OccurredAt: review.CreatedAt.UTC(),
	}, nil
}

type Outbox interface {
	Claim(context.Context, int, time.Time) ([]Message, error)
	MarkPublished(context.Context, uuid.UUID, time.Time) error
	MarkFailed(context.Context, uuid.UUID, string, time.Time) error
}

type Publisher interface {
	Publish(context.Context, Message) error
}
