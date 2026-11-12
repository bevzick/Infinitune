package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
)

const (
	ReleasePublishedV1 = "catalog.release.published.v1"
	ReleaseArchivedV1  = "catalog.release.archived.v1"
)

type Topics struct {
	ReleasePublished string
	ReleaseArchived  string
}

type Envelope[T any] struct {
	ID         uuid.UUID `json:"id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	OccurredAt time.Time `json:"occurred_at"`
	Data       T         `json:"data"`
}

type PublishedTrackData struct {
	CatalogTrackID uuid.UUID `json:"catalog_track_id"`
	ContentTrackID uuid.UUID `json:"content_track_id"`
}

type ReleasePublishedData struct {
	SourceReleaseID uuid.UUID            `json:"source_release_id"`
	ArtistID        uuid.UUID            `json:"artist_id"`
	AlbumID         *uuid.UUID           `json:"album_id,omitempty"`
	Tracks          []PublishedTrackData `json:"tracks"`
}

type ReleaseArchivedData struct {
	SourceReleaseID uuid.UUID   `json:"source_release_id"`
	ArtistID        uuid.UUID   `json:"artist_id"`
	AlbumID         *uuid.UUID  `json:"album_id,omitempty"`
	TrackIDs        []uuid.UUID `json:"track_ids"`
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

func NewReleasePublished(publication metadata.Publication, topic string) (Message, error) {
	tracks := make([]PublishedTrackData, 0, len(publication.Tracks))
	for _, track := range publication.Tracks {
		tracks = append(tracks, PublishedTrackData{
			CatalogTrackID: track.CatalogTrackID, ContentTrackID: track.ContentTrackID,
		})
	}
	return newMessage(
		publication.SourceReleaseID, ReleasePublishedV1, topic, publication.PublishedAt,
		ReleasePublishedData{
			SourceReleaseID: publication.SourceReleaseID, ArtistID: publication.ArtistID,
			AlbumID: publication.AlbumID, Tracks: tracks,
		},
	)
}

func NewReleaseArchived(
	publication metadata.Publication,
	archivedAt time.Time,
	topic string,
) (Message, error) {
	trackIDs := make([]uuid.UUID, 0, len(publication.Tracks))
	for _, track := range publication.Tracks {
		trackIDs = append(trackIDs, track.CatalogTrackID)
	}
	return newMessage(
		publication.SourceReleaseID, ReleaseArchivedV1, topic, archivedAt,
		ReleaseArchivedData{
			SourceReleaseID: publication.SourceReleaseID, ArtistID: publication.ArtistID,
			AlbumID: publication.AlbumID, TrackIDs: trackIDs,
		},
	)
}

func newMessage[T any](
	aggregateID uuid.UUID,
	eventType string,
	topic string,
	occurredAt time.Time,
	data T,
) (Message, error) {
	eventID := uuid.New()
	payload, err := json.Marshal(Envelope[T]{
		ID: eventID, Type: eventType, Source: "catalog-service",
		OccurredAt: occurredAt.UTC(), Data: data,
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: eventID, AggregateID: aggregateID, Type: eventType, Topic: topic,
		Key: aggregateID.String(), Payload: payload, OccurredAt: occurredAt.UTC(),
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
