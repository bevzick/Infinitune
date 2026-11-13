package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
)

const AssetReadyV1 = "media.asset.ready.v1"

type Envelope[T any] struct {
	ID         uuid.UUID `json:"id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       T         `json:"data"`
}

type AudioVariant struct {
	Codec       string `json:"codec"`
	Container   string `json:"container"`
	BitrateKbps int    `json:"bitrate_kbps"`
	SampleRate  int    `json:"sample_rate"`
	Channels    int    `json:"channels"`
}

type AssetReadyData struct {
	MediaAssetID   uuid.UUID      `json:"media_asset_id"`
	SourceUploadID *uuid.UUID     `json:"source_upload_id,omitempty"`
	ContentTrackID *uuid.UUID     `json:"content_track_id,omitempty"`
	CatalogTrackID *uuid.UUID     `json:"catalog_track_id,omitempty"`
	DurationMS     int            `json:"duration_ms"`
	Variants       []AudioVariant `json:"variants"`
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

func NewAssetReady(asset media.Asset, variants []media.AudioVariant, occurredAt time.Time, topic string) (Message, error) {
	eventID := uuid.New()
	eventVariants := make([]AudioVariant, 0, len(variants))
	for _, variant := range variants {
		eventVariants = append(eventVariants, AudioVariant{
			Codec: variant.Codec, Container: variant.Container,
			BitrateKbps: variant.BitrateKbps, SampleRate: variant.SampleRate,
			Channels: variant.Channels,
		})
	}
	durationMS := 0
	if asset.DurationMS != nil {
		durationMS = *asset.DurationMS
	}
	var sourceUploadID, contentTrackID, catalogTrackID *uuid.UUID
	if asset.SourceUploadID != uuid.Nil {
		value := asset.SourceUploadID
		sourceUploadID = &value
	}
	if asset.ContentTrackID != uuid.Nil {
		value := asset.ContentTrackID
		contentTrackID = &value
	}
	if asset.TrackID != uuid.Nil {
		value := asset.TrackID
		catalogTrackID = &value
	}
	payload, err := json.Marshal(Envelope[AssetReadyData]{
		ID: eventID, Type: AssetReadyV1, Source: "media-service", OccurredAt: occurredAt.UTC(),
		Data: AssetReadyData{
			MediaAssetID: asset.ID, SourceUploadID: sourceUploadID,
			ContentTrackID: contentTrackID, CatalogTrackID: catalogTrackID,
			DurationMS: durationMS, Variants: eventVariants,
		},
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: eventID, AggregateID: asset.ID, Type: AssetReadyV1, Topic: topic,
		Key: asset.ID.String(), Payload: payload, OccurredAt: occurredAt.UTC(),
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
