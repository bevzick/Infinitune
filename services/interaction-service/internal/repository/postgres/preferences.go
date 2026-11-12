package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/events"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *LikeRepository) LikeTrack(
	ctx context.Context,
	userID, trackID uuid.UUID,
) (interaction.Like, error) {
	return repository.preferenceLike(ctx, userID, trackID, events.TargetTrack, events.ActionLiked, `
		INSERT INTO interaction.track_likes (user_id, track_id)
		VALUES ($1,$2) ON CONFLICT (user_id,track_id) DO NOTHING
		RETURNING user_id,track_id,created_at
	`, `SELECT user_id,track_id,created_at FROM interaction.track_likes WHERE user_id=$1 AND track_id=$2`)
}

func (repository *LikeRepository) UnlikeTrack(ctx context.Context, userID, trackID uuid.UUID) error {
	return repository.preferenceUnlike(ctx, userID, trackID, events.TargetTrack, events.ActionUnliked, `
		DELETE FROM interaction.track_likes
		WHERE user_id=$1 AND track_id=$2 RETURNING user_id,track_id
	`)
}

func (repository *LikeRepository) LikeAlbum(
	ctx context.Context,
	userID, albumID uuid.UUID,
) (interaction.Like, error) {
	return repository.preferenceLike(ctx, userID, albumID, events.TargetAlbum, events.ActionLiked, `
		INSERT INTO interaction.album_likes (user_id, album_id)
		VALUES ($1,$2) ON CONFLICT (user_id,album_id) DO NOTHING
		RETURNING user_id,album_id,created_at
	`, `SELECT user_id,album_id,created_at FROM interaction.album_likes WHERE user_id=$1 AND album_id=$2`)
}

func (repository *LikeRepository) UnlikeAlbum(ctx context.Context, userID, albumID uuid.UUID) error {
	return repository.preferenceUnlike(ctx, userID, albumID, events.TargetAlbum, events.ActionUnliked, `
		DELETE FROM interaction.album_likes
		WHERE user_id=$1 AND album_id=$2 RETURNING user_id,album_id
	`)
}

func (repository *LikeRepository) FollowArtist(
	ctx context.Context,
	userID, artistID uuid.UUID,
) (interaction.Follow, error) {
	liked, err := repository.preferenceLike(
		ctx, userID, artistID, events.TargetArtist, events.ActionFollowed, `
			INSERT INTO interaction.artist_follows (user_id, artist_id)
			VALUES ($1,$2) ON CONFLICT (user_id,artist_id) DO NOTHING
			RETURNING user_id,artist_id,created_at
		`, `SELECT user_id,artist_id,created_at FROM interaction.artist_follows WHERE user_id=$1 AND artist_id=$2`,
	)
	if err != nil {
		return interaction.Follow{}, err
	}
	return interaction.Follow{
		UserID: liked.UserID, ArtistID: liked.TargetID, FollowedAt: liked.CreatedAt,
	}, nil
}

func (repository *LikeRepository) UnfollowArtist(ctx context.Context, userID, artistID uuid.UUID) error {
	return repository.preferenceUnlike(ctx, userID, artistID, events.TargetArtist, events.ActionUnfollowed, `
		DELETE FROM interaction.artist_follows
		WHERE user_id=$1 AND artist_id=$2 RETURNING user_id,artist_id
	`)
}

func (repository *LikeRepository) preferenceLike(
	ctx context.Context,
	userID, targetID uuid.UUID,
	targetType events.TargetType,
	action events.Action,
	insertQuery, existingQuery string,
) (interaction.Like, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return interaction.Like{}, fmt.Errorf("begin preference change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var result interaction.Like
	err = tx.QueryRow(ctx, insertQuery, userID, targetID).Scan(
		&result.UserID, &result.TargetID, &result.CreatedAt,
	)
	changed := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, existingQuery, userID, targetID).Scan(
			&result.UserID, &result.TargetID, &result.CreatedAt,
		)
	}
	if err != nil {
		return interaction.Like{}, fmt.Errorf("store %s preference: %w", targetType, err)
	}
	if changed {
		message, messageErr := events.NewPreferenceChanged(
			userID, targetType, targetID, action, result.CreatedAt, repository.topic,
		)
		if messageErr != nil {
			return interaction.Like{}, fmt.Errorf("encode preference event: %w", messageErr)
		}
		if err = enqueuePreferenceEvent(ctx, tx, message); err != nil {
			return interaction.Like{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return interaction.Like{}, fmt.Errorf("commit preference change: %w", err)
	}
	return result, nil
}

func (repository *LikeRepository) preferenceUnlike(
	ctx context.Context,
	userID, targetID uuid.UUID,
	targetType events.TargetType,
	action events.Action,
	deleteQuery string,
) error {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin preference removal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var removedUserID, removedTargetID uuid.UUID
	err = tx.QueryRow(ctx, deleteQuery, userID, targetID).Scan(&removedUserID, &removedTargetID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit idempotent preference removal: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove %s preference: %w", targetType, err)
	}
	var occurredAt time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&occurredAt); err != nil {
		return fmt.Errorf("timestamp preference removal: %w", err)
	}
	message, err := events.NewPreferenceChanged(
		removedUserID, targetType, removedTargetID, action, occurredAt, repository.topic,
	)
	if err != nil {
		return fmt.Errorf("encode preference removal event: %w", err)
	}
	if err = enqueuePreferenceEvent(ctx, tx, message); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit preference removal: %w", err)
	}
	return nil
}

func enqueuePreferenceEvent(ctx context.Context, tx pgx.Tx, message events.Message) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO interaction.event_outbox (
			id, aggregate_id, event_type, topic, partition_key, payload, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, message.ID, message.AggregateID, message.Type, message.Topic,
		message.Key, message.Payload, message.OccurredAt)
	if err != nil {
		return fmt.Errorf("enqueue preference event: %w", err)
	}
	return nil
}
