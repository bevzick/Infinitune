//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/database"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMetadataRepositoryStoresAndReplacesCompleteMetadata(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewMetadataRepository(pool)
	service := metadata.NewService(repository)
	ctx := context.Background()
	testID := strings.ReplaceAll(uuid.NewString(), "-", "")

	primaryID := uuid.New()
	featuredID := uuid.New()
	producerID := uuid.New()
	genreOneID := uuid.New()
	genreTwoID := uuid.New()
	moodOneID := uuid.New()
	moodTwoID := uuid.New()
	albumID := uuid.New()
	trackID := uuid.New()
	registerCleanup(t, pool, []uuid.UUID{trackID}, []uuid.UUID{albumID},
		[]uuid.UUID{primaryID, featuredID, producerID},
		[]uuid.UUID{genreOneID, genreTwoID}, []uuid.UUID{moodOneID, moodTwoID})

	for _, artist := range []metadata.ArtistInput{
		{ID: primaryID, Name: "Primary " + testID},
		{ID: featuredID, Name: "Featured " + testID},
		{ID: producerID, Name: "Producer " + testID},
	} {
		if _, err := service.StoreApprovedArtist(ctx, artist); err != nil {
			t.Fatalf("StoreApprovedArtist(%s) error = %v", artist.Name, err)
		}
	}
	updatedArtist, err := service.StoreApprovedArtist(ctx, metadata.ArtistInput{
		ID: primaryID, Name: "Updated Primary " + testID,
		Description: "Updated approved biography",
		ImageURL:    "https://cdn.example.test/artist.jpg",
	})
	if err != nil {
		t.Fatalf("StoreApprovedArtist(update) error = %v", err)
	}
	if updatedArtist.Description != "Updated approved biography" || updatedArtist.ImageURL == "" {
		t.Fatalf("updated artist = %+v", updatedArtist)
	}
	for _, genre := range []metadata.GenreInput{
		{ID: genreOneID, Name: "Genre One " + testID, Slug: "genre-one-" + testID},
		{ID: genreTwoID, Name: "Genre Two " + testID, Slug: "genre-two-" + testID},
	} {
		if _, err := service.StoreGenre(ctx, genre); err != nil {
			t.Fatalf("StoreGenre(%s) error = %v", genre.Name, err)
		}
	}
	for _, mood := range []metadata.MoodInput{
		{ID: moodOneID, Name: "Mood One " + testID, Slug: "mood-one-" + testID},
		{ID: moodTwoID, Name: "Mood Two " + testID, Slug: "mood-two-" + testID},
	} {
		if _, err := service.StoreMood(ctx, mood); err != nil {
			t.Fatalf("StoreMood(%s) error = %v", mood.Name, err)
		}
	}
	readGenre, err := service.Genre(ctx, genreOneID)
	if err != nil || readGenre.ID != genreOneID {
		t.Fatalf("Genre() = %+v, %v", readGenre, err)
	}
	readMood, err := service.Mood(ctx, moodOneID)
	if err != nil || readMood.ID != moodOneID {
		t.Fatalf("Mood() = %+v, %v", readMood, err)
	}

	releaseDate := time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC)
	album, err := service.StoreApprovedAlbum(ctx, metadata.AlbumInput{
		ID: albumID, Title: "Shared Single", Type: metadata.AlbumTypeSingle,
		ReleaseDate: releaseDate,
		Artists: []metadata.ArtistPositionInput{
			{ArtistID: primaryID, Position: 1},
			{ArtistID: featuredID, Position: 2},
		},
	})
	if err != nil {
		t.Fatalf("StoreApprovedAlbum() error = %v", err)
	}
	if len(album.Artists) != 2 || album.Artists[0].Artist.ID != primaryID ||
		album.Artists[1].Artist.ID != featuredID {
		t.Fatalf("album artists = %+v", album.Artists)
	}

	track, err := service.StoreApprovedTrack(ctx, metadata.TrackInput{
		ID: trackID, AlbumID: &albumID, Title: "Collaborative Track", DurationMS: 210000,
		TrackNumber: 1, DiscNumber: 1, ReleaseDate: releaseDate,
		MediaAssetID: uuid.New(), ContentID: uuid.New(),
		Artists: []metadata.TrackArtistInput{
			{ArtistID: primaryID, Role: metadata.ArtistRolePrimary, Position: 1},
			{ArtistID: featuredID, Role: metadata.ArtistRoleFeatured, Position: 1},
			{ArtistID: producerID, Role: metadata.ArtistRoleProducer, Position: 1},
			{ArtistID: primaryID, Role: metadata.ArtistRoleComposer, Position: 1},
		},
		GenreIDs: []uuid.UUID{genreOneID, genreTwoID},
		Moods: []metadata.TrackMoodInput{
			{MoodID: moodOneID, Weight: 0.8},
			{MoodID: moodTwoID, Weight: 0.4},
		},
	})
	if err != nil {
		t.Fatalf("StoreApprovedTrack() error = %v", err)
	}
	if len(track.Artists) != 4 || len(track.Genres) != 2 || len(track.Moods) != 2 {
		t.Fatalf("complete track = artists %d, genres %d, moods %d", len(track.Artists), len(track.Genres), len(track.Moods))
	}
	for index, role := range []metadata.ArtistRole{
		metadata.ArtistRolePrimary,
		metadata.ArtistRoleFeatured,
		metadata.ArtistRoleProducer,
		metadata.ArtistRoleComposer,
	} {
		if track.Artists[index].Role != role {
			t.Errorf("track artist %d role = %q, want %q", index, track.Artists[index].Role, role)
		}
	}

	page := metadata.PageRequest{Limit: 100, Offset: 0}
	artistsPage, err := service.Artists(ctx, page)
	if err != nil {
		t.Fatalf("Artists() error = %v", err)
	}
	if artistsPage.Total < 3 || !containsArtist(artistsPage.Items, primaryID) {
		t.Fatalf("Artists() = %+v", artistsPage)
	}
	albumsPage, err := service.Albums(ctx, page)
	if err != nil {
		t.Fatalf("Albums() error = %v", err)
	}
	listedAlbum, exists := findAlbum(albumsPage.Items, albumID)
	if !exists || len(listedAlbum.Artists) != 2 {
		t.Fatalf("Albums() album = %+v, exists = %t", listedAlbum, exists)
	}
	tracksPage, err := service.Tracks(ctx, page)
	if err != nil {
		t.Fatalf("Tracks() error = %v", err)
	}
	listedTrack, exists := findTrack(tracksPage.Items, trackID)
	if !exists || len(listedTrack.Artists) != 4 || len(listedTrack.Genres) != 2 || len(listedTrack.Moods) != 2 {
		t.Fatalf("Tracks() track = %+v, exists = %t", listedTrack, exists)
	}
	genresPage, err := service.Genres(ctx, page)
	if err != nil || genresPage.Total < 2 || !containsGenre(genresPage.Items, genreOneID) {
		t.Fatalf("Genres() = %+v, %v", genresPage, err)
	}
	moodsPage, err := service.Moods(ctx, page)
	if err != nil || moodsPage.Total < 2 || !containsMood(moodsPage.Items, moodOneID) {
		t.Fatalf("Moods() = %+v, %v", moodsPage, err)
	}
	emptyPage, err := service.Tracks(ctx, metadata.PageRequest{Limit: 20, Offset: 100000})
	if err != nil {
		t.Fatalf("Tracks(empty page) error = %v", err)
	}
	if emptyPage.Items == nil || len(emptyPage.Items) != 0 || emptyPage.Total < 1 {
		t.Fatalf("Tracks(empty page) = %+v", emptyPage)
	}

	updatedAlbum, err := service.StoreApprovedAlbum(ctx, metadata.AlbumInput{
		ID: albumID, Title: "Updated Shared Single", Type: metadata.AlbumTypeSingle,
		ReleaseDate: releaseDate,
		Artists:     []metadata.ArtistPositionInput{{ArtistID: featuredID, Position: 1}},
	})
	if err != nil {
		t.Fatalf("StoreApprovedAlbum(update) error = %v", err)
	}
	if updatedAlbum.Title != "Updated Shared Single" || len(updatedAlbum.Artists) != 1 ||
		updatedAlbum.Artists[0].Artist.ID != featuredID {
		t.Fatalf("updated album = %+v", updatedAlbum)
	}

	updatedTrack, err := service.StoreApprovedTrack(ctx, metadata.TrackInput{
		ID: trackID, Title: "Standalone Edit", DurationMS: 205000,
		TrackNumber: 1, DiscNumber: 1, Explicit: true, ReleaseDate: releaseDate,
		MediaAssetID: track.MediaAssetID, ContentID: track.ContentID,
		Artists: []metadata.TrackArtistInput{
			{ArtistID: featuredID, Role: metadata.ArtistRolePrimary, Position: 1},
		},
		GenreIDs: []uuid.UUID{genreTwoID},
		Moods:    []metadata.TrackMoodInput{{MoodID: moodTwoID, Weight: 1}},
	})
	if err != nil {
		t.Fatalf("StoreApprovedTrack(update) error = %v", err)
	}
	if updatedTrack.AlbumID != nil || updatedTrack.Title != "Standalone Edit" || !updatedTrack.Explicit ||
		len(updatedTrack.Artists) != 1 || updatedTrack.Artists[0].Artist.ID != featuredID ||
		len(updatedTrack.Genres) != 1 || updatedTrack.Genres[0].ID != genreTwoID ||
		len(updatedTrack.Moods) != 1 || updatedTrack.Moods[0].Mood.ID != moodTwoID {
		t.Fatalf("updated track = %+v", updatedTrack)
	}

	readTrack, err := service.Track(ctx, trackID)
	if err != nil {
		t.Fatalf("Track() error = %v", err)
	}
	if readTrack.ID != updatedTrack.ID || readTrack.Title != updatedTrack.Title {
		t.Fatalf("Track() = %+v, want %+v", readTrack, updatedTrack)
	}
}

func TestMetadataRepositoryMapsReferenceAndConflictErrors(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewMetadataRepository(pool)
	service := metadata.NewService(repository)
	ctx := context.Background()
	testID := strings.ReplaceAll(uuid.NewString(), "-", "")
	genreOneID := uuid.New()
	genreTwoID := uuid.New()
	failedAlbumID := uuid.New()
	registerCleanup(t, pool, nil, []uuid.UUID{failedAlbumID}, nil,
		[]uuid.UUID{genreOneID, genreTwoID}, nil)

	_, err := service.StoreApprovedAlbum(ctx, metadata.AlbumInput{
		ID: failedAlbumID, Title: "Missing Artist", Type: metadata.AlbumTypeSingle,
		ReleaseDate: time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
		Artists:     []metadata.ArtistPositionInput{{ArtistID: uuid.New(), Position: 1}},
	})
	if !errors.Is(err, metadata.ErrInvalidReference) {
		t.Fatalf("StoreApprovedAlbum() error = %v, want %v", err, metadata.ErrInvalidReference)
	}
	if _, err = service.Album(ctx, failedAlbumID); !errors.Is(err, metadata.ErrNotFound) {
		t.Fatalf("Album(after rollback) error = %v, want %v", err, metadata.ErrNotFound)
	}

	name := "Conflict Genre " + testID
	if _, err = service.StoreGenre(ctx, metadata.GenreInput{
		ID: genreOneID, Name: name, Slug: "conflict-one-" + testID,
	}); err != nil {
		t.Fatalf("StoreGenre(first) error = %v", err)
	}
	_, err = service.StoreGenre(ctx, metadata.GenreInput{
		ID: genreTwoID, Name: strings.ToUpper(name), Slug: "conflict-two-" + testID,
	})
	if !errors.Is(err, metadata.ErrConflict) {
		t.Fatalf("StoreGenre(duplicate) error = %v, want %v", err, metadata.ErrConflict)
	}
	if _, err = service.Artist(ctx, uuid.New()); !errors.Is(err, metadata.ErrNotFound) {
		t.Fatalf("Artist(missing) error = %v, want %v", err, metadata.ErrNotFound)
	}
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}

	port, err := strconv.ParseUint(envOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatalf("parse POSTGRES_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, database.Config{
		Host:     envOrDefault("POSTGRES_HOST", "127.0.0.1"),
		Port:     uint16(port),
		User:     requiredEnv(t, "POSTGRES_USER"),
		Password: requiredEnv(t, "POSTGRES_PASSWORD"),
		Name:     requiredEnv(t, "POSTGRES_CATALOG_DB"),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func registerCleanup(
	t *testing.T,
	pool *pgxpool.Pool,
	trackIDs []uuid.UUID,
	albumIDs []uuid.UUID,
	artistIDs []uuid.UUID,
	genreIDs []uuid.UUID,
	moodIDs []uuid.UUID,
) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, trackID := range trackIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.track_moods WHERE track_id = $1`, trackID)
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.track_genres WHERE track_id = $1`, trackID)
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.track_artists WHERE track_id = $1`, trackID)
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.tracks WHERE id = $1`, trackID)
		}
		for _, albumID := range albumIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.album_artists WHERE album_id = $1`, albumID)
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.albums WHERE id = $1`, albumID)
		}
		for _, artistID := range artistIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.artists WHERE id = $1`, artistID)
		}
		for _, genreID := range genreIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.genres WHERE id = $1`, genreID)
		}
		for _, moodID := range moodIDs {
			_, _ = pool.Exec(ctx, `DELETE FROM catalog.moods WHERE id = $1`, moodID)
		}
	})
}

func requiredEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func containsArtist(items []metadata.Artist, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func findAlbum(items []metadata.Album, id uuid.UUID) (metadata.Album, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return metadata.Album{}, false
}

func findTrack(items []metadata.Track, id uuid.UUID) (metadata.Track, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return metadata.Track{}, false
}

func containsGenre(items []metadata.Genre, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func containsMood(items []metadata.Mood, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
