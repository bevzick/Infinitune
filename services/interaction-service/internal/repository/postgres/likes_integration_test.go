//go:build integration

package postgres

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLikeRepositoryIsIdempotentAndIsolatesOwners(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewLikeRepository(pool)
	ctx := context.Background()
	ownerID, strangerID := uuid.New(), uuid.New()
	trackID, albumID, commentID := uuid.New(), uuid.New(), uuid.New()
	firstArtistID, secondArtistID := uuid.New(), uuid.New()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM interaction.track_likes WHERE user_id = ANY($1::uuid[])`,
			[]uuid.UUID{ownerID, strangerID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM interaction.album_likes WHERE user_id = ANY($1::uuid[])`,
			[]uuid.UUID{ownerID, strangerID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM interaction.artist_follows WHERE user_id = ANY($1::uuid[])`,
			[]uuid.UUID{ownerID, strangerID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM interaction.comment_likes WHERE user_id = ANY($1::uuid[])`,
			[]uuid.UUID{ownerID, strangerID})
	})

	first, err := repository.LikeTrack(ctx, ownerID, trackID)
	if err != nil {
		t.Fatalf("LikeTrack() error = %v", err)
	}
	second, err := repository.LikeTrack(ctx, ownerID, trackID)
	if err != nil || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("LikeTrack(idempotent) = %+v, %v; first = %+v", second, err, first)
	}
	if _, err = repository.LikeAlbum(ctx, ownerID, albumID); err != nil {
		t.Fatalf("LikeAlbum() error = %v", err)
	}
	commentLike, err := repository.LikeComment(ctx, ownerID, commentID)
	if err != nil {
		t.Fatalf("LikeComment() error = %v", err)
	}
	again, err := repository.LikeComment(ctx, ownerID, commentID)
	if err != nil || !again.CreatedAt.Equal(commentLike.CreatedAt) {
		t.Fatalf("LikeComment(idempotent) = %+v, %v; first = %+v", again, err, commentLike)
	}
	if _, err = repository.LikeAlbum(ctx, ownerID, albumID); err != nil {
		t.Fatalf("LikeAlbum(idempotent) error = %v", err)
	}

	if err = repository.UnlikeTrack(ctx, strangerID, trackID); err != nil {
		t.Fatalf("UnlikeTrack(stranger) error = %v", err)
	}
	tracks, err := repository.TrackLikes(ctx, ownerID, interaction.PageRequest{Limit: 10})
	if err != nil || tracks.Total != 1 || len(tracks.Items) != 1 || tracks.Items[0].TargetID != trackID {
		t.Fatalf("TrackLikes(owner) = %+v, %v", tracks, err)
	}
	albums, err := repository.AlbumLikes(ctx, strangerID, interaction.PageRequest{Limit: 10})
	if err != nil || albums.Total != 0 || len(albums.Items) != 0 {
		t.Fatalf("AlbumLikes(stranger) = %+v, %v", albums, err)
	}
	if err = repository.UnlikeComment(ctx, strangerID, commentID); err != nil {
		t.Fatalf("UnlikeComment(stranger) error = %v", err)
	}
	comments, err := repository.CommentLikes(ctx, ownerID, interaction.PageRequest{Limit: 10})
	if err != nil || comments.Total != 1 || len(comments.Items) != 1 || comments.Items[0].TargetID != commentID {
		t.Fatalf("CommentLikes(owner) = %+v, %v", comments, err)
	}

	for _, artistID := range []uuid.UUID{firstArtistID, secondArtistID} {
		first, followErr := repository.FollowArtist(ctx, ownerID, artistID)
		if followErr != nil {
			t.Fatalf("FollowArtist(%s) error = %v", artistID, followErr)
		}
		again, followErr := repository.FollowArtist(ctx, ownerID, artistID)
		if followErr != nil || !again.FollowedAt.Equal(first.FollowedAt) {
			t.Fatalf("FollowArtist(%s, idempotent) = %+v, %v; first = %+v",
				artistID, again, followErr, first)
		}
	}
	if err = repository.UnfollowArtist(ctx, strangerID, firstArtistID); err != nil {
		t.Fatalf("UnfollowArtist(stranger) error = %v", err)
	}
	follows, err := repository.ArtistFollows(ctx, ownerID, interaction.PageRequest{Limit: 10})
	if err != nil || follows.Total != 2 || len(follows.Items) != 2 ||
		follows.Items[0].ArtistID == follows.Items[1].ArtistID {
		t.Fatalf("ArtistFollows(owner) = %+v, %v", follows, err)
	}

	for range 2 {
		if err = repository.UnlikeTrack(ctx, ownerID, trackID); err != nil {
			t.Fatalf("UnlikeTrack(idempotent) error = %v", err)
		}
		if err = repository.UnlikeAlbum(ctx, ownerID, albumID); err != nil {
			t.Fatalf("UnlikeAlbum(idempotent) error = %v", err)
		}
		if err = repository.UnfollowArtist(ctx, ownerID, firstArtistID); err != nil {
			t.Fatalf("UnfollowArtist(idempotent) error = %v", err)
		}
		if err = repository.UnlikeComment(ctx, ownerID, commentID); err != nil {
			t.Fatalf("UnlikeComment(idempotent) error = %v", err)
		}
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
	config, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	config.ConnConfig.Host = envOrDefault("POSTGRES_HOST", "127.0.0.1")
	config.ConnConfig.Port = uint16(port)
	config.ConnConfig.User = requiredEnv(t, "POSTGRES_USER")
	config.ConnConfig.Password = requiredEnv(t, "POSTGRES_PASSWORD")
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_INTERACTION_DB")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping postgres: %v", err)
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
