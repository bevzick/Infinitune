//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/database"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPlaylistRepositoryLifecycle(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewPlaylistRepository(pool)
	service := playlist.NewService(repository)
	ctx := context.Background()
	ownerID := uuid.New()

	created, err := service.Create(ctx, ownerID, playlist.CreateInput{
		Name:        "Integration playlist",
		Description: "created by repository integration test",
		Visibility:  playlist.VisibilityUnlisted,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1`, created.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlists WHERE id = $1`, created.ID)
	})
	if created.OwnerID != ownerID || created.Type != playlist.TypeManual ||
		created.Visibility != playlist.VisibilityUnlisted || created.CreatedAt.IsZero() {
		t.Fatalf("Create() = %+v", created)
	}

	read, err := service.ByID(ctx, uuid.Nil, created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if read != created {
		t.Fatalf("ByID() = %+v, want %+v", read, created)
	}

	name := "Updated integration playlist"
	visibility := playlist.VisibilityPrivate
	updated, err := service.Update(ctx, ownerID, created.ID, playlist.UpdateInput{
		Name:       &name,
		Visibility: &visibility,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.OwnerID != ownerID || updated.Name != name ||
		updated.Visibility != playlist.VisibilityPrivate {
		t.Fatalf("Update() = %+v", updated)
	}

	trackID := uuid.New()
	if _, err = pool.Exec(ctx, `
		INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
		VALUES ($1, $2, 1, $3)
	`, created.ID, trackID, ownerID); err != nil {
		t.Fatalf("insert playlist track before Delete(): %v", err)
	}
	if err = service.Delete(ctx, ownerID, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err = repository.ByID(ctx, created.ID); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("ByID() after Delete() error = %v, want %v", err, playlist.ErrNotFound)
	}
	var trackCount int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM playlist.playlist_tracks WHERE playlist_id = $1
	`, created.ID).Scan(&trackCount); err != nil {
		t.Fatalf("count tracks after Delete(): %v", err)
	}
	if trackCount != 0 {
		t.Fatalf("track count after Delete() = %d, want 0", trackCount)
	}
}

func TestPlaylistRepositoryRequiresMatchingOwnerForMutation(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewPlaylistRepository(pool)
	ctx := context.Background()
	ownerID := uuid.New()
	created, err := repository.Create(ctx, playlist.Playlist{
		ID: uuid.New(), OwnerID: ownerID, Name: "Owned playlist",
		Type: playlist.TypeManual, Visibility: playlist.VisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1`, created.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlists WHERE id = $1`, created.ID)
	})

	created.OwnerID = uuid.New()
	created.Name = "Unauthorized update"
	trackID := uuid.New()
	if _, err = pool.Exec(ctx, `
		INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
		VALUES ($1, $2, 1, $3)
	`, created.ID, trackID, ownerID); err != nil {
		t.Fatalf("insert playlist track: %v", err)
	}
	if _, err = repository.Update(ctx, created); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("Update() with wrong owner error = %v, want %v", err, playlist.ErrNotFound)
	}
	if err = repository.Delete(ctx, created.ID, created.OwnerID); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("Delete() with wrong owner error = %v, want %v", err, playlist.ErrNotFound)
	}

	read, err := repository.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if read.OwnerID != ownerID || read.Name != "Owned playlist" {
		t.Fatalf("playlist changed after unauthorized mutations: %+v", read)
	}
	var trackCount int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM playlist.playlist_tracks WHERE playlist_id = $1
	`, created.ID).Scan(&trackCount); err != nil {
		t.Fatalf("count tracks after unauthorized Delete(): %v", err)
	}
	if trackCount != 1 {
		t.Fatalf("track count after unauthorized Delete() = %d, want 1", trackCount)
	}
}

func TestPlaylistRepositoryManagesManualTrackOrder(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewPlaylistRepository(pool)
	playlists := playlist.NewService(repository)
	tracks := playlist.NewTrackService(playlists, repository, acceptingTrackValidator{})
	ctx := context.Background()
	ownerID := uuid.New()
	created, err := playlists.Create(ctx, ownerID, playlist.CreateInput{Name: "Ordered playlist"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1`, created.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlists WHERE id = $1`, created.ID)
	})

	firstID := uuid.New()
	secondID := uuid.New()
	first, err := tracks.Add(ctx, ownerID, created.ID, firstID)
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	second, err := tracks.Add(ctx, ownerID, created.ID, secondID)
	if err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	if first.Position != 1 || second.Position != 2 {
		t.Fatalf("added positions = %d, %d; want 1, 2", first.Position, second.Position)
	}
	if _, err = tracks.Add(ctx, ownerID, created.ID, firstID); !errors.Is(err, playlist.ErrTrackExists) {
		t.Fatalf("Add(duplicate) error = %v, want %v", err, playlist.ErrTrackExists)
	}

	if _, err = tracks.Reorder(ctx, ownerID, created.ID, []uuid.UUID{secondID}); !errors.Is(err, playlist.ErrInvalidArgument) {
		t.Fatalf("Reorder(incomplete) error = %v, want %v", err, playlist.ErrInvalidArgument)
	}
	unchanged, err := tracks.Tracks(ctx, ownerID, created.ID, playlist.PageRequest{})
	if err != nil {
		t.Fatalf("Tracks(after rejected reorder) error = %v", err)
	}
	assertTrackOrder(t, unchanged.Items, []uuid.UUID{firstID, secondID})
	if unchanged.Total != 2 || unchanged.Limit != 50 {
		t.Fatalf("Tracks() page = %+v", unchanged)
	}

	reordered, err := tracks.Reorder(ctx, ownerID, created.ID, []uuid.UUID{secondID, firstID})
	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	assertTrackOrder(t, reordered, []uuid.UUID{secondID, firstID})
	if reordered[0].AddedAt != second.AddedAt || reordered[1].AddedAt != first.AddedAt {
		t.Fatal("Reorder() did not preserve track metadata")
	}

	if err = tracks.Remove(ctx, ownerID, created.ID, secondID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	remaining, err := tracks.Tracks(ctx, ownerID, created.ID, playlist.PageRequest{Limit: 1})
	if err != nil {
		t.Fatalf("Tracks(after remove) error = %v", err)
	}
	assertTrackOrder(t, remaining.Items, []uuid.UUID{firstID})
	if err = tracks.Remove(ctx, ownerID, created.ID, secondID); !errors.Is(err, playlist.ErrTrackNotFound) {
		t.Fatalf("Remove(missing) error = %v, want %v", err, playlist.ErrTrackNotFound)
	}
}

func TestPlaylistRepositoryDefendsTrackMutations(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewPlaylistRepository(pool)
	ctx := context.Background()
	ownerID := uuid.New()
	dynamic, err := repository.Create(ctx, playlist.Playlist{
		ID: uuid.New(), OwnerID: ownerID, Name: "Dynamic playlist",
		Type: playlist.TypeDynamic, Visibility: playlist.VisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1`, dynamic.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlists WHERE id = $1`, dynamic.ID)
	})

	if _, err = repository.AddTrack(ctx, dynamic.ID, uuid.New(), uuid.New(), ownerID); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("AddTrack(wrong owner) error = %v, want %v", err, playlist.ErrNotFound)
	}
	if _, err = repository.AddTrack(ctx, dynamic.ID, ownerID, uuid.New(), ownerID); !errors.Is(err, playlist.ErrManualOnly) {
		t.Fatalf("AddTrack(dynamic) error = %v, want %v", err, playlist.ErrManualOnly)
	}
}

func TestPlaylistRepositoryDynamicCriteriaLifecycle(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewPlaylistRepository(pool)
	playlists := playlist.NewService(repository)
	dynamic := playlist.NewDynamicService(playlists, repository)
	ctx := context.Background()
	ownerID := uuid.New()
	artistIDs := []uuid.UUID{uuid.New(), uuid.New()}

	created, err := dynamic.Create(ctx, ownerID, playlist.DynamicCreateInput{
		Playlist: playlist.CreateInput{
			Name:        "Dynamic discoveries",
			Description: "created by repository integration test",
			Visibility:  playlist.VisibilityUnlisted,
		},
		Criteria: rules.CreateInput{
			Name:  "Recent clean tracks",
			Logic: rules.LogicAnd,
			Conditions: []rules.ConditionInput{
				{Field: rules.FieldTitle, Operator: rules.OperatorContains, Value: rules.TextValue("live")},
				{Field: rules.FieldArtistID, Operator: rules.OperatorIn, Value: rules.UUIDListValue(artistIDs...)},
				{Field: rules.FieldReleaseYear, Operator: rules.OperatorBetween, Value: rules.IntegerRangeValue(2020, 2026)},
				{Field: rules.FieldExplicit, Operator: rules.OperatorEquals, Value: rules.BooleanValue(false)},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1`, created.Playlist.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlists WHERE id = $1`, created.Playlist.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM rule_engine.rule_sets WHERE id = $1`, created.RuleSet.ID)
	})
	if created.Playlist.Type != playlist.TypeDynamic ||
		created.Playlist.RuleSetID == nil || *created.Playlist.RuleSetID != created.RuleSet.ID ||
		created.RuleSet.OwnerID != ownerID || created.RuleSet.CreatedAt.IsZero() {
		t.Fatalf("Create() = %+v", created)
	}

	read, err := dynamic.Criteria(ctx, ownerID, created.Playlist.ID)
	if err != nil {
		t.Fatalf("Criteria() error = %v", err)
	}
	if read.ID != created.RuleSet.ID || read.Name != created.RuleSet.Name ||
		read.Logic != created.RuleSet.Logic || !reflect.DeepEqual(read.Conditions, created.RuleSet.Conditions) {
		t.Fatalf("Criteria() = %+v, want %+v", read, created.RuleSet)
	}

	unauthorized := read
	unauthorized.OwnerID = uuid.New()
	if _, err = repository.ReplaceRuleSet(ctx, unauthorized); !errors.Is(err, rules.ErrNotFound) {
		t.Fatalf("ReplaceRuleSet() with wrong owner error = %v, want %v", err, rules.ErrNotFound)
	}

	replaced, err := dynamic.ReplaceCriteria(ctx, ownerID, created.Playlist.ID, rules.CreateInput{
		Name:  "Long or classic tracks",
		Logic: rules.LogicOr,
		Conditions: []rules.ConditionInput{
			{Field: rules.FieldDurationMS, Operator: rules.OperatorGreaterThan, Value: rules.IntegerValue(300_000)},
			{Field: rules.FieldReleaseYear, Operator: rules.OperatorLessThan, Value: rules.IntegerValue(1990)},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceCriteria() error = %v", err)
	}
	if replaced.ID != created.RuleSet.ID || replaced.OwnerID != ownerID ||
		replaced.Name != "Long or classic tracks" || replaced.Logic != rules.LogicOr ||
		len(replaced.Conditions) != 2 || !replaced.CreatedAt.Equal(created.RuleSet.CreatedAt) {
		t.Fatalf("ReplaceCriteria() = %+v", replaced)
	}

	persisted, err := dynamic.Criteria(ctx, ownerID, created.Playlist.ID)
	if err != nil {
		t.Fatalf("Criteria() after replacement error = %v", err)
	}
	if !reflect.DeepEqual(persisted, replaced) {
		t.Fatalf("persisted criteria = %+v, want %+v", persisted, replaced)
	}
	var trackCount int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM playlist.playlist_tracks WHERE playlist_id = $1
	`, created.Playlist.ID).Scan(&trackCount); err != nil {
		t.Fatalf("count dynamic playlist tracks: %v", err)
	}
	if trackCount != 0 {
		t.Fatalf("dynamic playlist track count = %d, want 0", trackCount)
	}

	if err = playlists.Delete(ctx, ownerID, created.Playlist.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err = repository.ByID(ctx, created.Playlist.ID); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("ByID() after Delete() error = %v, want %v", err, playlist.ErrNotFound)
	}
	if _, err = repository.RuleSetByID(ctx, created.RuleSet.ID); !errors.Is(err, rules.ErrNotFound) {
		t.Fatalf("RuleSetByID() after Delete() error = %v, want %v", err, rules.ErrNotFound)
	}
}

func assertTrackOrder(t *testing.T, tracks []playlist.Track, want []uuid.UUID) {
	t.Helper()
	if len(tracks) != len(want) {
		t.Fatalf("track count = %d, want %d; tracks = %+v", len(tracks), len(want), tracks)
	}
	for index, trackID := range want {
		if tracks[index].TrackID != trackID || tracks[index].Position != index+1 {
			t.Fatalf("tracks[%d] = %+v, want id %s at position %d",
				index, tracks[index], trackID, index+1)
		}
	}
}

type acceptingTrackValidator struct{}

func (acceptingTrackValidator) ValidateTrack(context.Context, uuid.UUID) error { return nil }

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
		User:     requiredEnvironment(t, "POSTGRES_USER"),
		Password: requiredEnvironment(t, "POSTGRES_PASSWORD"),
		Name:     requiredEnvironment(t, "POSTGRES_PLAYLIST_DB"),
	})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func requiredEnvironment(t *testing.T, key string) string {
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
