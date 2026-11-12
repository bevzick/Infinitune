package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const PreferenceChangedV1 = "interaction.preference.changed.v1"

type TargetType string
type Action string

const (
	TargetTrack  TargetType = "track"
	TargetAlbum  TargetType = "album"
	TargetArtist TargetType = "artist"

	ActionLiked      Action = "liked"
	ActionUnliked    Action = "unliked"
	ActionFollowed   Action = "followed"
	ActionUnfollowed Action = "unfollowed"
)

type Envelope[T any] struct {
	ID         uuid.UUID `json:"id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       T         `json:"data"`
}

type PreferenceChangedData struct {
	UserID     uuid.UUID  `json:"user_id"`
	TargetType TargetType `json:"target_type"`
	TargetID   uuid.UUID  `json:"target_id"`
	Action     Action     `json:"action"`
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

func NewPreferenceChanged(
	userID uuid.UUID,
	targetType TargetType,
	targetID uuid.UUID,
	action Action,
	occurredAt time.Time,
	topic string,
) (Message, error) {
	eventID := uuid.New()
	payload, err := json.Marshal(Envelope[PreferenceChangedData]{
		ID: eventID, Type: PreferenceChangedV1, Source: "interaction-service",
		OccurredAt: occurredAt.UTC(),
		Data: PreferenceChangedData{
			UserID: userID, TargetType: targetType, TargetID: targetID, Action: action,
		},
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: eventID, AggregateID: userID, Type: PreferenceChangedV1, Topic: topic,
		Key: userID.String(), Payload: payload, OccurredAt: occurredAt.UTC(),
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
