package postgres

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LikeRepository struct {
	database *pgxpool.Pool
	topic    string
}

func NewLikeRepository(database *pgxpool.Pool, configuredTopic ...string) *LikeRepository {
	topic := "interaction.preference.changed.v1"
	if len(configuredTopic) > 0 {
		topic = configuredTopic[0]
	}
	return &LikeRepository{database: database, topic: topic}
}

func (repository *LikeRepository) TrackLikes(
	ctx context.Context, userID uuid.UUID, page interaction.PageRequest,
) (interaction.LikePage, error) {
	return repository.list(ctx, userID, page, `
		SELECT count(*) FROM interaction.track_likes WHERE user_id = $1
	`, `
		SELECT user_id, track_id, created_at
		FROM interaction.track_likes
		WHERE user_id = $1
		ORDER BY created_at DESC, track_id DESC
		LIMIT $2 OFFSET $3
	`, "track")
}

func (repository *LikeRepository) AlbumLikes(
	ctx context.Context, userID uuid.UUID, page interaction.PageRequest,
) (interaction.LikePage, error) {
	return repository.list(ctx, userID, page, `
		SELECT count(*) FROM interaction.album_likes WHERE user_id = $1
	`, `
		SELECT user_id, album_id, created_at
		FROM interaction.album_likes
		WHERE user_id = $1
		ORDER BY created_at DESC, album_id DESC
		LIMIT $2 OFFSET $3
	`, "album")
}

func (repository *LikeRepository) LikeComment(
	ctx context.Context, userID, commentID uuid.UUID,
) (interaction.Like, error) {
	return repository.upsert(ctx, `
		INSERT INTO interaction.comment_likes (user_id, comment_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, comment_id) DO UPDATE SET user_id = EXCLUDED.user_id
		RETURNING user_id, comment_id, created_at
	`, userID, commentID, "comment")
}

func (repository *LikeRepository) UnlikeComment(ctx context.Context, userID, commentID uuid.UUID) error {
	return repository.remove(ctx, `
		DELETE FROM interaction.comment_likes WHERE user_id = $1 AND comment_id = $2
	`, userID, commentID, "comment")
}

func (repository *LikeRepository) CommentLikes(
	ctx context.Context, userID uuid.UUID, page interaction.PageRequest,
) (interaction.LikePage, error) {
	return repository.list(ctx, userID, page, `
		SELECT count(*) FROM interaction.comment_likes WHERE user_id = $1
	`, `
		SELECT user_id, comment_id, created_at
		FROM interaction.comment_likes
		WHERE user_id = $1
		ORDER BY created_at DESC, comment_id DESC
		LIMIT $2 OFFSET $3
	`, "comment")
}

func (repository *LikeRepository) upsert(
	ctx context.Context, query string, userID, targetID uuid.UUID, target string,
) (interaction.Like, error) {
	var result interaction.Like
	err := repository.database.QueryRow(ctx, query, userID, targetID).Scan(
		&result.UserID, &result.TargetID, &result.CreatedAt,
	)
	if err != nil {
		return interaction.Like{}, fmt.Errorf("upsert %s like: %w", target, err)
	}
	return result, nil
}

func (repository *LikeRepository) remove(
	ctx context.Context, query string, userID, targetID uuid.UUID, target string,
) error {
	if _, err := repository.database.Exec(ctx, query, userID, targetID); err != nil {
		return fmt.Errorf("delete %s like: %w", target, err)
	}
	return nil
}

func (repository *LikeRepository) list(
	ctx context.Context,
	userID uuid.UUID,
	page interaction.PageRequest,
	countQuery string,
	itemsQuery string,
	target string,
) (interaction.LikePage, error) {
	var total int64
	if err := repository.database.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return interaction.LikePage{}, fmt.Errorf("count %s likes: %w", target, err)
	}
	rows, err := repository.database.Query(ctx, itemsQuery, userID, page.Limit, page.Offset)
	if err != nil {
		return interaction.LikePage{}, fmt.Errorf("query %s likes: %w", target, err)
	}
	defer rows.Close()
	items, err := scanLikes(rows)
	if err != nil {
		return interaction.LikePage{}, fmt.Errorf("scan %s likes: %w", target, err)
	}
	return interaction.LikePage{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func scanLikes(rows pgx.Rows) ([]interaction.Like, error) {
	result := make([]interaction.Like, 0)
	for rows.Next() {
		var like interaction.Like
		if err := rows.Scan(&like.UserID, &like.TargetID, &like.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, like)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

var _ interaction.Store = (*LikeRepository)(nil)
