//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRunRepositoryPersistsAndReplaysVersionedRun(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewRunRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID, firstTrack, secondTrack := uuid.New(), uuid.New(), uuid.New()
	run, err := recommendation.NewRun(
		uuid.New(),
		userID,
		"transparent-v1",
		now,
		now.Add(time.Hour),
		[]recommendation.Item{
			{TrackID: secondTrack, Score: 0.5, Rank: 2, Reason: "recent artist"},
			{TrackID: firstTrack, Score: 0.9, Rank: 1, Reason: "preferred genre"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM recommendation.recommendation_items WHERE run_id = $1`, run.ID)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM recommendation.recommendation_runs WHERE id = $1`, run.ID)
	})
	stored, err := repository.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	stored.Items[0].Reason = "mutated caller copy"
	read, err := repository.ByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if read.AlgorithmVersion != "transparent-v1" || len(read.Items) != 2 ||
		read.Items[0].TrackID != firstTrack || read.Items[0].Rank != 1 ||
		read.Items[0].Reason != "preferred genre" || read.Items[1].TrackID != secondTrack {
		t.Fatalf("ByID() = %+v", read)
	}
	latest, err := repository.LatestValid(ctx, userID, "transparent-v1", now.Add(30*time.Minute))
	if err != nil || latest.ID != run.ID {
		t.Fatalf("LatestValid() = %+v, %v", latest, err)
	}
	if _, err = repository.LatestValid(ctx, userID, "transparent-v1", now.Add(2*time.Hour)); !errors.Is(err, recommendation.ErrNotFound) {
		t.Fatalf("LatestValid() expired error = %v, want %v", err, recommendation.ErrNotFound)
	}
	if _, err = repository.Save(ctx, run); !errors.Is(err, recommendation.ErrConflict) {
		t.Fatalf("duplicate Save() error = %v, want %v", err, recommendation.ErrConflict)
	}
	candidate, err := recommendation.NewRun(
		uuid.New(), userID, "transparent-v1", now.Add(time.Minute), now.Add(61*time.Minute),
		[]recommendation.Item{{TrackID: uuid.New(), Score: 0.7, Rank: 1, Reason: "preferred artist"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM recommendation.recommendation_items WHERE run_id = $1`, candidate.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM recommendation.recommendation_runs WHERE id = $1`, candidate.ID)
	})
	winner, created, err := repository.SaveIfNoValid(ctx, candidate, now.Add(30*time.Minute))
	if err != nil || created || winner.ID != run.ID {
		t.Fatalf("SaveIfNoValid(cached) = %+v, %v, %v", winner, created, err)
	}
	future, err := recommendation.NewRun(
		candidate.ID, userID, "transparent-v1", now.Add(2*time.Hour), now.Add(3*time.Hour), candidate.Items,
	)
	if err != nil {
		t.Fatal(err)
	}
	winner, created, err = repository.SaveIfNoValid(ctx, future, now.Add(2*time.Hour))
	if err != nil || !created || winner.ID != future.ID {
		t.Fatalf("SaveIfNoValid(expired) = %+v, %v, %v", winner, created, err)
	}
}

func TestRunRepositoryRollsBackRunWhenAnItemFails(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewRunRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	runID := uuid.New()
	_, err := repository.Save(context.Background(), recommendation.Run{
		ID: runID, UserID: uuid.New(), AlgorithmVersion: "transparent-v1",
		GeneratedAt: now, ExpiresAt: now.Add(time.Hour),
		Items: []recommendation.Item{
			{RunID: runID, TrackID: uuid.New(), Score: 0.9, Rank: 1, Reason: "genre"},
			{RunID: runID, TrackID: uuid.New(), Score: 0.8, Rank: 1, Reason: "artist"},
		},
	})
	if err == nil {
		t.Fatal("Save() error = nil, want duplicate rank error")
	}
	var count int
	if queryErr := pool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM recommendation.recommendation_runs
		WHERE id = $1
	`, runID).Scan(&count); queryErr != nil {
		t.Fatal(queryErr)
	}
	if count != 0 {
		t.Fatalf("recommendation run count = %d, want rollback", count)
	}
}

func TestInvalidateBeforeExpiresOnlyOlderRuns(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewRunRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	eventTime := time.Now().UTC().Truncate(time.Microsecond)
	older, err := recommendation.NewRun(
		uuid.New(), userID, "transparent-v1", eventTime.Add(-time.Minute),
		eventTime.Add(time.Hour), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	newer, err := recommendation.NewRun(
		uuid.New(), userID, "transparent-v1", eventTime.Add(time.Minute),
		eventTime.Add(2*time.Hour), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range []recommendation.Run{older, newer} {
		if _, err = repository.Save(ctx, run); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		runID := run.ID
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(),
				`DELETE FROM recommendation.recommendation_runs WHERE id=$1`, runID)
		})
	}
	updated, err := repository.InvalidateBefore(ctx, userID, eventTime)
	if err != nil || updated != 1 {
		t.Fatalf("InvalidateBefore() = %d, %v", updated, err)
	}
	storedOlder, err := repository.ByID(ctx, older.ID)
	if err != nil || !storedOlder.ExpiresAt.Equal(eventTime) {
		t.Fatalf("older run = %+v, %v", storedOlder, err)
	}
	storedNewer, err := repository.ByID(ctx, newer.ID)
	if err != nil || !storedNewer.ExpiresAt.Equal(newer.ExpiresAt) {
		t.Fatalf("newer run = %+v, %v", storedNewer, err)
	}
}

func TestCatalogInvalidationTargetsAffectedRuns(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewRunRepository(pool)
	ctx := context.Background()
	eventTime := time.Now().UTC().Truncate(time.Microsecond)
	archivedTrack, otherTrack := uuid.New(), uuid.New()
	newRun := func(generatedAt time.Time, trackID uuid.UUID) recommendation.Run {
		run, err := recommendation.NewRun(
			uuid.New(), uuid.New(), "transparent-v1", generatedAt,
			generatedAt.Add(2*time.Hour),
			[]recommendation.Item{{TrackID: trackID, Score: 1, Rank: 1, Reason: "test"}},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = repository.Save(ctx, run); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(),
				`DELETE FROM recommendation.recommendation_runs WHERE id=$1`, run.ID)
		})
		return run
	}
	olderAffected := newRun(eventTime.Add(-time.Minute), archivedTrack)
	olderUnrelated := newRun(eventTime.Add(-time.Minute), otherTrack)
	newerAffected := newRun(eventTime.Add(time.Minute), archivedTrack)

	updated, err := repository.InvalidateTracksBefore(
		ctx, []uuid.UUID{archivedTrack}, eventTime,
	)
	if err != nil || updated != 1 {
		t.Fatalf("InvalidateTracksBefore() = %d, %v", updated, err)
	}
	stored, err := repository.ByID(ctx, olderAffected.ID)
	if err != nil || !stored.ExpiresAt.Equal(eventTime) {
		t.Fatalf("older affected run = %+v, %v", stored, err)
	}
	stored, err = repository.ByID(ctx, newerAffected.ID)
	if err != nil || !stored.ExpiresAt.Equal(newerAffected.ExpiresAt) {
		t.Fatalf("newer affected run = %+v, %v", stored, err)
	}

	updated, err = repository.InvalidateAllBefore(ctx, eventTime)
	if err != nil || updated != 1 {
		t.Fatalf("InvalidateAllBefore() = %d, %v", updated, err)
	}
	stored, err = repository.ByID(ctx, olderUnrelated.ID)
	if err != nil || !stored.ExpiresAt.Equal(eventTime) {
		t.Fatalf("older unrelated run = %+v, %v", stored, err)
	}
}

func TestRunRepositorySerializesConcurrentGeneration(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewRunRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := uuid.New()
	candidates := make([]recommendation.Run, 2)
	for index := range candidates {
		var err error
		candidates[index], err = recommendation.NewRun(
			uuid.New(), userID, "transparent-v1", now, now.Add(time.Hour),
			[]recommendation.Item{{TrackID: uuid.New(), Score: float64(2 - index), Rank: 1, Reason: "preferred genre"}},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, candidate := range candidates {
			_, _ = pool.Exec(context.Background(), `DELETE FROM recommendation.recommendation_items WHERE run_id = $1`, candidate.ID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM recommendation.recommendation_runs WHERE id = $1`, candidate.ID)
		}
	})
	type saveResult struct {
		run     recommendation.Run
		created bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan saveResult, len(candidates))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, candidate := range candidates {
		go func(run recommendation.Run) {
			<-start
			stored, created, err := repository.SaveIfNoValid(ctx, run, now)
			results <- saveResult{run: stored, created: created, err: err}
		}(candidate)
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("SaveIfNoValid() errors = %v, %v", first.err, second.err)
	}
	if first.run.ID != second.run.ID {
		t.Fatalf("concurrent runs differ: %s != %s", first.run.ID, second.run.ID)
	}
	createdCount := 0
	if first.created {
		createdCount++
	}
	if second.created {
		createdCount++
	}
	if createdCount != 1 {
		t.Fatalf("created count = %d, want 1", createdCount)
	}
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1")
	}
	port, err := strconv.ParseUint(envOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Host = envOrDefault("POSTGRES_HOST", "127.0.0.1")
	config.ConnConfig.Port = uint16(port)
	config.ConnConfig.User = requiredEnv(t, "POSTGRES_USER")
	config.ConnConfig.Password = requiredEnv(t, "POSTGRES_PASSWORD")
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_RECOMMENDATION_DB")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
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
