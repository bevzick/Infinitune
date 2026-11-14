package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
)

const (
	ListeningRecordedV1 = "history.listening.recorded.v1"
	ListeningClearedV1  = "history.listening.cleared.v1"
)

type Envelope[T any] struct {
	ID         uuid.UUID `json:"id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       T         `json:"data"`
}

type ListeningRecordedData struct {
	UserID      uuid.UUID  `json:"user_id"`
	TrackID     uuid.UUID  `json:"track_id"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	ListenedMS  int        `json:"listened_ms"`
	Completed   bool       `json:"completed"`
	ContextType string     `json:"context_type"`
	ContextID   *uuid.UUID `json:"context_id,omitempty"`
	DeviceType  string     `json:"device_type"`
}

type ListeningClearedData struct {
	UserID       uuid.UUID  `json:"user_id"`
	Before       *time.Time `json:"before,omitempty"`
	DeletedCount int64      `json:"deleted_count"`
}

type Message struct {
	ID         uuid.UUID
	Type       string
	Topic      string
	Key        string
	Payload    []byte
	OccurredAt time.Time
	Attempts   int
}

func NewListeningRecorded(event history.Event, topic string) (Message, error) {
	occurredAt := event.StartedAt
	if event.FinishedAt != nil {
		occurredAt = *event.FinishedAt
	}
	payload, err := json.Marshal(Envelope[ListeningRecordedData]{
		ID: event.ID, Type: ListeningRecordedV1, Source: "history-service",
		OccurredAt: occurredAt,
		Data: ListeningRecordedData{
			UserID: event.UserID, TrackID: event.TrackID, StartedAt: event.StartedAt,
			FinishedAt: event.FinishedAt, ListenedMS: event.ListenedMS,
			Completed: event.Completed, ContextType: event.ContextType,
			ContextID: event.ContextID, DeviceType: event.DeviceType,
		},
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: event.ID, Type: ListeningRecordedV1, Topic: topic,
		Key: event.UserID.String(), Payload: payload, OccurredAt: occurredAt,
	}, nil
}

func NewListeningCleared(
	eventID uuid.UUID,
	userID uuid.UUID,
	before *time.Time,
	deletedCount int64,
	clearedAt time.Time,
	topic string,
) (Message, error) {
	data := ListeningClearedData{UserID: userID, DeletedCount: deletedCount}
	if before != nil {
		value := before.UTC()
		data.Before = &value
	}
	payload, err := json.Marshal(Envelope[ListeningClearedData]{
		ID: eventID, Type: ListeningClearedV1, Source: "history-service",
		OccurredAt: clearedAt.UTC(), Data: data,
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: eventID, Type: ListeningClearedV1, Topic: topic,
		Key: userID.String(), Payload: payload, OccurredAt: clearedAt.UTC(),
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
