//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/events"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublicationRepositoryPublishesAtomicallyAndIdempotently(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	sourceID, artistID, albumID := uuid.New(), uuid.New(), uuid.New()
	trackOneID, trackTwoID := uuid.New(), uuid.New()
	contentOneID, contentTwoID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		cleanupPublication(t, pool, sourceID, []uuid.UUID{trackOneID, trackTwoID}, &albumID, artistID)
	})
	releaseDate := time.Date(2027, 4, 5, 0, 0, 0, 0, time.UTC)
	input := metadata.PublicationInput{
		SourceReleaseID: sourceID,
		Artist:          metadata.ArtistInput{ID: artistID, Name: "Published Artist"},
		Album: &metadata.AlbumInput{
			ID: albumID, Title: "Published Album", Type: metadata.AlbumTypeAlbum,
			ReleaseDate: releaseDate,
			Artists:     []metadata.ArtistPositionInput{{ArtistID: artistID, Position: 1}},
		},
		Tracks: []metadata.TrackInput{
			publicationTrack(trackOneID, contentOneID, albumID, artistID, releaseDate, 1),
			publicationTrack(trackTwoID, contentTwoID, albumID, artistID, releaseDate, 2),
		},
	}
	service := metadata.NewService(NewMetadataRepository(pool))
	published, err := service.PublishApprovedRelease(ctx, input)
	if err != nil || published.SourceReleaseID != sourceID ||
		published.AlbumID == nil || *published.AlbumID != albumID ||
		len(published.Tracks) != 2 || published.PublishedAt.IsZero() {
		t.Fatalf("PublishApprovedRelease() = %+v, %v", published, err)
	}
	albumValidation, err := service.ValidateAlbumIDs(ctx, []uuid.UUID{albumID, uuid.New()})
	if err != nil || len(albumValidation.ValidIDs) != 1 || albumValidation.ValidIDs[0] != albumID ||
		len(albumValidation.MissingIDs) != 1 {
		t.Fatalf("ValidateAlbumIDs() = %+v, %v", albumValidation, err)
	}
	artistValidation, err := service.ValidateArtistIDs(ctx, []uuid.UUID{artistID, uuid.New()})
	if err != nil || len(artistValidation.ValidIDs) != 1 || artistValidation.ValidIDs[0] != artistID ||
		len(artistValidation.MissingIDs) != 1 {
		t.Fatalf("ValidateArtistIDs() = %+v, %v", artistValidation, err)
	}
	again, err := service.PublishApprovedRelease(ctx, input)
	if err != nil || !again.PublishedAt.Equal(published.PublishedAt) || len(again.Tracks) != 2 {
		t.Fatalf("PublishApprovedRelease(idempotent) = %+v, %v", again, err)
	}
	input.Tracks[0].Title = "Different payload"
	if _, err = service.PublishApprovedRelease(ctx, input); !errors.Is(err, metadata.ErrConflict) {
		t.Fatalf("PublishApprovedRelease(conflict) error = %v", err)
	}
	var publications, albums, tracks int
	if err = pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM catalog.publications WHERE source_release_id = $1),
			(SELECT count(*) FROM catalog.albums WHERE id = $2),
			(SELECT count(*) FROM catalog.tracks WHERE id = ANY($3::uuid[]))
	`, sourceID, albumID, []uuid.UUID{trackOneID, trackTwoID}).Scan(
		&publications, &albums, &tracks,
	); err != nil || publications != 1 || albums != 1 || tracks != 2 {
		t.Fatalf("catalog counts = publications %d, albums %d, tracks %d; error = %v",
			publications, albums, tracks, err)
	}
	archived, err := service.ArchivePublishedRelease(ctx, sourceID)
	if err != nil || archived.SourceReleaseID != sourceID || archived.ArchivedAt.IsZero() {
		t.Fatalf("ArchivePublishedRelease() = %+v, %v", archived, err)
	}
	againArchived, err := service.ArchivePublishedRelease(ctx, sourceID)
	if err != nil || !againArchived.ArchivedAt.Equal(archived.ArchivedAt) {
		t.Fatalf("ArchivePublishedRelease(idempotent) = %+v, %v", againArchived, err)
	}
	if _, err = service.Track(ctx, trackOneID); !errors.Is(err, metadata.ErrNotFound) {
		t.Fatalf("Track(archived) error = %v", err)
	}
	batch, err := service.BatchTracks(ctx, []uuid.UUID{trackOneID, trackTwoID})
	if err != nil || len(batch) != 2 {
		t.Fatalf("BatchTracks(archived) = %d, %v", len(batch), err)
	}
	validation, err := service.ValidateTrackIDs(ctx, []uuid.UUID{trackOneID, trackTwoID})
	if err != nil || len(validation.ValidIDs) != 0 || len(validation.MissingIDs) != 2 {
		t.Fatalf("ValidateTrackIDs(archived) = %+v, %v", validation, err)
	}
	if _, err = service.Album(ctx, albumID); !errors.Is(err, metadata.ErrNotFound) {
		t.Fatalf("Album(archived) error = %v", err)
	}
	albumValidation, err = service.ValidateAlbumIDs(ctx, []uuid.UUID{albumID})
	if err != nil || len(albumValidation.ValidIDs) != 0 || len(albumValidation.MissingIDs) != 1 {
		t.Fatalf("ValidateAlbumIDs(archived) = %+v, %v", albumValidation, err)
	}
	artistValidation, err = service.ValidateArtistIDs(ctx, []uuid.UUID{artistID})
	if err != nil || len(artistValidation.ValidIDs) != 1 || artistValidation.ValidIDs[0] != artistID {
		t.Fatalf("ValidateArtistIDs(after archive) = %+v, %v", artistValidation, err)
	}
	var publishedEvents, archivedEvents int
	if err = pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE event_type=$2),
			count(*) FILTER (WHERE event_type=$3)
		FROM catalog.event_outbox
		WHERE aggregate_id=$1
	`, sourceID, events.ReleasePublishedV1, events.ReleaseArchivedV1).Scan(
		&publishedEvents, &archivedEvents,
	); err != nil || publishedEvents != 1 || archivedEvents != 1 {
		t.Fatalf("outbox events = published %d, archived %d; error = %v",
			publishedEvents, archivedEvents, err)
	}
}

func publicationTrack(
	id, contentID, albumID, artistID uuid.UUID,
	releaseDate time.Time,
	number int16,
) metadata.TrackInput {
	return metadata.TrackInput{
		ID: id, AlbumID: &albumID, Title: "Track", DurationMS: 180000,
		TrackNumber: number, DiscNumber: 1, ReleaseDate: releaseDate,
		MediaAssetID: uuid.New(), ContentID: contentID,
		Artists: []metadata.TrackArtistInput{{
			ArtistID: artistID, Role: metadata.ArtistRolePrimary, Position: 1,
		}},
	}
}

func cleanupPublication(
	t *testing.T,
	pool *pgxpool.Pool,
	sourceID uuid.UUID,
	trackIDs []uuid.UUID,
	albumID *uuid.UUID,
	artistID uuid.UUID,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = pool.Exec(ctx, `DELETE FROM catalog.event_outbox WHERE aggregate_id = $1`, sourceID)
	_, _ = pool.Exec(ctx, `DELETE FROM catalog.publications WHERE source_release_id = $1`, sourceID)
	for _, trackID := range trackIDs {
		_, _ = pool.Exec(ctx, `DELETE FROM catalog.track_artists WHERE track_id = $1`, trackID)
		_, _ = pool.Exec(ctx, `DELETE FROM catalog.tracks WHERE id = $1`, trackID)
	}
	if albumID != nil {
		_, _ = pool.Exec(ctx, `DELETE FROM catalog.album_artists WHERE album_id = $1`, *albumID)
		_, _ = pool.Exec(ctx, `DELETE FROM catalog.albums WHERE id = $1`, *albumID)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM catalog.artists WHERE id = $1`, artistID)
}
