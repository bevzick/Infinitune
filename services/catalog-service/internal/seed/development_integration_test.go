//go:build integration

package seed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/database"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	lumenHarborID  = uuid.MustParse("10000000-0000-4000-8000-000000000001")
	firstLightID   = uuid.MustParse("30000000-0000-4000-8000-000000000001")
	seededTrackIDs = []uuid.UUID{
		firstLightID,
		uuid.MustParse("30000000-0000-4000-8000-000000000002"),
		uuid.MustParse("30000000-0000-4000-8000-000000000003"),
		uuid.MustParse("30000000-0000-4000-8000-000000000004"),
		uuid.MustParse("30000000-0000-4000-8000-000000000005"),
		uuid.MustParse("30000000-0000-4000-8000-000000000006"),
	}
)

func TestDevelopmentSeedIsIdempotentAndBrowsable(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}

	seedSQL := readDevelopmentSeed(t)
	pool := openIntegrationDatabase(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin seed transaction: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
	before := snapshotSeedTaxonomy(t, tx)

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = tx.Exec(context.Background(), seedSQL); err != nil {
			t.Fatalf("apply development seed attempt %d: %v", attempt, err)
		}
	}

	assertSeedTaxonomyUnchanged(t, tx, before)
	assertSeedCounts(t, tx)
	assertSeedBrowseAndSearch(t, tx)
}

func TestDevelopmentSeedReusesPreexistingTaxonomy(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}

	seedSQL := readDevelopmentSeed(t)
	pool := openIntegrationDatabase(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin seed transaction: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})

	if _, err = tx.Exec(context.Background(), `
		DELETE FROM catalog.track_genres
		WHERE track_id = ANY($1)
		  AND genre_id IN (
			SELECT id FROM catalog.genres
			WHERE lower(name) = 'electronic' OR lower(slug) = 'electronic'
		  )
	`, seededTrackIDs); err != nil {
		t.Fatalf("remove existing seeded electronic relationships: %v", err)
	}
	if _, err = tx.Exec(context.Background(), `
		UPDATE catalog.genres
		SET name = 'Preserved ' || left(replace(id::text, '-', ''), 12),
		    slug = 'preserved-' || left(replace(id::text, '-', ''), 12)
		WHERE lower(name) = 'electronic' OR lower(slug) = 'electronic'
	`); err != nil {
		t.Fatalf("reserve electronic taxonomy name: %v", err)
	}

	preexistingID := uuid.New()
	preexistingSlug := "preexisting-electronic-" + preexistingID.String()[:8]
	if _, err = tx.Exec(context.Background(), `
		INSERT INTO catalog.genres (id, name, slug) VALUES ($1, 'Electronic', $2)
	`, preexistingID, preexistingSlug); err != nil {
		t.Fatalf("insert pre-existing electronic genre: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = tx.Exec(context.Background(), seedSQL); err != nil {
			t.Fatalf("apply development seed with pre-existing taxonomy attempt %d: %v", attempt, err)
		}
	}

	var name, slug string
	if err = tx.QueryRow(context.Background(), `
		SELECT name, slug FROM catalog.genres WHERE id = $1
	`, preexistingID).Scan(&name, &slug); err != nil {
		t.Fatalf("read pre-existing electronic genre: %v", err)
	}
	if name != "Electronic" || slug != preexistingSlug {
		t.Fatalf("seed changed pre-existing electronic genre from %q/%q to %q/%q",
			"Electronic", preexistingSlug, name, slug)
	}

	var relationshipCount int
	if err = tx.QueryRow(context.Background(), `
		SELECT count(*) FROM catalog.track_genres
		WHERE track_id = ANY($1) AND genre_id = $2
	`, seededTrackIDs, preexistingID).Scan(&relationshipCount); err != nil {
		t.Fatalf("count reused electronic relationships: %v", err)
	}
	if relationshipCount != 4 {
		t.Fatalf("reused electronic relationships = %d, want 4", relationshipCount)
	}
}

func readDevelopmentSeed(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve seed test path")
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "seeds", "development.sql"))
	if err != nil {
		t.Fatalf("read development seed: %v", err)
	}
	return string(contents)
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
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

func assertSeedCounts(t *testing.T, tx pgx.Tx) {
	t.Helper()
	tests := []struct {
		name      string
		query     string
		arguments []any
		want      int
	}{
		{
			"artists",
			`SELECT count(*) FROM catalog.artists WHERE id::text LIKE '10000000-0000-4000-8000-%'`,
			nil,
			3,
		},
		{
			"albums",
			`SELECT count(*) FROM catalog.albums WHERE id::text LIKE '20000000-0000-4000-8000-%'`,
			nil,
			2,
		},
		{"tracks", `SELECT count(*) FROM catalog.tracks WHERE id = ANY($1)`, []any{seededTrackIDs}, 6},
		{
			"genres", semanticTaxonomyCountSQL("genres"),
			[]any{seedTaxonomyNames("genres"), seedTaxonomySlugs("genres")}, 4,
		},
		{
			"moods", semanticTaxonomyCountSQL("moods"),
			[]any{seedTaxonomyNames("moods"), seedTaxonomySlugs("moods")}, 4,
		},
		{"track artists", `SELECT count(*) FROM catalog.track_artists WHERE track_id = ANY($1)`, []any{seededTrackIDs}, 10},
		{"track genres", `SELECT count(*) FROM catalog.track_genres WHERE track_id = ANY($1)`, []any{seededTrackIDs}, 12},
		{"track moods", `SELECT count(*) FROM catalog.track_moods WHERE track_id = ANY($1)`, []any{seededTrackIDs}, 12},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got int
			if err := tx.QueryRow(context.Background(), test.query, test.arguments...).Scan(&got); err != nil {
				t.Fatalf("count seeded %s: %v", test.name, err)
			}
			if got != test.want {
				t.Fatalf("seeded %s count = %d, want %d", test.name, got, test.want)
			}
		})
	}
}

func assertSeedBrowseAndSearch(t *testing.T, tx pgx.Tx) {
	t.Helper()
	service := metadata.NewService(postgres.NewMetadataRepository(tx))
	ctx := context.Background()

	tracks, err := service.Tracks(ctx, metadata.TrackQuery{
		Page: metadata.PageRequest{Limit: 100}, Sort: metadata.TrackSortNewest,
	})
	if err != nil {
		t.Fatalf("browse seeded tracks: %v", err)
	}
	if !containsTrack(tracks.Items, firstLightID) {
		t.Fatalf("browse tracks does not contain seeded track %s", firstLightID)
	}

	releaseYear := 2025
	notExplicit := false
	var synthwaveID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT id FROM catalog.genres
		WHERE lower(slug) = 'synthwave' OR lower(name) = 'synthwave'
		ORDER BY (lower(slug) = 'synthwave') DESC, id
		LIMIT 1
	`).Scan(&synthwaveID); err != nil {
		t.Fatalf("resolve seeded synthwave genre: %v", err)
	}
	result, err := service.Tracks(ctx, metadata.TrackQuery{
		Page:        metadata.PageRequest{Limit: 20},
		Title:       "first light",
		ArtistID:    &lumenHarborID,
		GenreID:     &synthwaveID,
		ReleaseYear: &releaseYear,
		Explicit:    &notExplicit,
		Sort:        metadata.TrackSortTitleAsc,
	})
	if err != nil {
		t.Fatalf("search seeded tracks: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != firstLightID {
		t.Fatalf("seeded track search = %+v, want First Light Engine", result)
	}
}

type taxonomySnapshot struct {
	table string
	id    uuid.UUID
	name  string
	slug  string
}

func snapshotSeedTaxonomy(t *testing.T, tx pgx.Tx) []taxonomySnapshot {
	t.Helper()
	result := make([]taxonomySnapshot, 0)
	for _, table := range []string{"genres", "moods"} {
		rows, err := tx.Query(context.Background(), fmt.Sprintf(`
			SELECT id, name, slug FROM catalog.%s
			WHERE lower(name) = ANY($1) OR lower(slug) = ANY($2)
			ORDER BY id
		`, table), seedTaxonomyNames(table), seedTaxonomySlugs(table))
		if err != nil {
			t.Fatalf("snapshot seed %s: %v", table, err)
		}
		for rows.Next() {
			value := taxonomySnapshot{table: table}
			if err = rows.Scan(&value.id, &value.name, &value.slug); err != nil {
				rows.Close()
				t.Fatalf("scan seed %s snapshot: %v", table, err)
			}
			result = append(result, value)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("iterate seed %s snapshot: %v", table, err)
		}
		rows.Close()
	}
	return result
}

func assertSeedTaxonomyUnchanged(t *testing.T, tx pgx.Tx, before []taxonomySnapshot) {
	t.Helper()
	for _, value := range before {
		var name, slug string
		query := fmt.Sprintf(`SELECT name, slug FROM catalog.%s WHERE id = $1`, value.table)
		if err := tx.QueryRow(context.Background(), query, value.id).Scan(&name, &slug); err != nil {
			t.Fatalf("read pre-existing %s %s: %v", value.table, value.id, err)
		}
		if name != value.name || slug != value.slug {
			t.Fatalf("seed changed pre-existing %s %s from %q/%q to %q/%q",
				value.table, value.id, value.name, value.slug, name, slug)
		}
	}
}

func semanticTaxonomyCountSQL(table string) string {
	return fmt.Sprintf(`
		SELECT count(*) FROM unnest($1::text[], $2::text[]) AS seed(name, slug)
		WHERE EXISTS (
			SELECT 1 FROM catalog.%s AS taxonomy
			WHERE lower(taxonomy.slug) = seed.slug OR lower(taxonomy.name) = seed.name
		)
	`, table)
}

func seedTaxonomyNames(table string) []string {
	if table == "genres" {
		return []string{"ambient", "electronic", "indie", "synthwave"}
	}
	return []string{"focused", "hopeful", "nocturnal", "energetic"}
}

func seedTaxonomySlugs(table string) []string {
	return seedTaxonomyNames(table)
}

func containsTrack(tracks []metadata.Track, id uuid.UUID) bool {
	for _, track := range tracks {
		if track.ID == id {
			return true
		}
	}
	return false
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
