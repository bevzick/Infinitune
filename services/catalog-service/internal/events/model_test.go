package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
)

func TestReleaseLifecycleMessagesContainCanonicalIDs(t *testing.T) {
	t.Parallel()

	sourceID, artistID, albumID := uuid.New(), uuid.New(), uuid.New()
	trackID, contentID := uuid.New(), uuid.New()
	publishedAt := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	publication := metadata.Publication{
		SourceReleaseID: sourceID, ArtistID: artistID, AlbumID: &albumID,
		PublishedAt: publishedAt,
		Tracks: []metadata.PublishedTrack{{
			CatalogTrackID: trackID, ContentTrackID: contentID,
		}},
	}
	published, err := NewReleasePublished(publication, "catalog-published")
	if err != nil {
		t.Fatalf("NewReleasePublished() error = %v", err)
	}
	if published.AggregateID != sourceID || published.Type != ReleasePublishedV1 ||
		published.Topic != "catalog-published" || published.Key != sourceID.String() {
		t.Fatalf("published message = %+v", published)
	}
	var publishedEnvelope Envelope[ReleasePublishedData]
	if err = json.Unmarshal(published.Payload, &publishedEnvelope); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if publishedEnvelope.Data.ArtistID != artistID ||
		len(publishedEnvelope.Data.Tracks) != 1 ||
		publishedEnvelope.Data.Tracks[0].CatalogTrackID != trackID {
		t.Fatalf("published envelope = %+v", publishedEnvelope)
	}

	archivedAt := publishedAt.Add(time.Hour)
	archived, err := NewReleaseArchived(publication, archivedAt, "catalog-archived")
	if err != nil {
		t.Fatalf("NewReleaseArchived() error = %v", err)
	}
	var archivedEnvelope Envelope[ReleaseArchivedData]
	if err = json.Unmarshal(archived.Payload, &archivedEnvelope); err != nil {
		t.Fatalf("decode archived payload: %v", err)
	}
	if archived.AggregateID != sourceID || archived.Type != ReleaseArchivedV1 ||
		len(archivedEnvelope.Data.TrackIDs) != 1 || archivedEnvelope.Data.TrackIDs[0] != trackID {
		t.Fatalf("archived message = %+v; envelope = %+v", archived, archivedEnvelope)
	}
}
