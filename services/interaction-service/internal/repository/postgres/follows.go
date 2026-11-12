package postgres

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
)

func (repository *LikeRepository) ArtistFollows(
	ctx context.Context,
	userID uuid.UUID,
	page interaction.PageRequest,
) (interaction.FollowPage, error) {
	var total int64
	if err := repository.database.QueryRow(ctx, `
		SELECT count(*) FROM interaction.artist_follows WHERE user_id = $1
	`, userID).Scan(&total); err != nil {
		return interaction.FollowPage{}, fmt.Errorf("count artist follows: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT user_id, artist_id, created_at
		FROM interaction.artist_follows
		WHERE user_id = $1
		ORDER BY created_at DESC, artist_id DESC
		LIMIT $2 OFFSET $3
	`, userID, page.Limit, page.Offset)
	if err != nil {
		return interaction.FollowPage{}, fmt.Errorf("query artist follows: %w", err)
	}
	defer rows.Close()
	items := make([]interaction.Follow, 0)
	for rows.Next() {
		var item interaction.Follow
		if err = rows.Scan(&item.UserID, &item.ArtistID, &item.FollowedAt); err != nil {
			return interaction.FollowPage{}, fmt.Errorf("scan artist follow: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return interaction.FollowPage{}, fmt.Errorf("iterate artist follows: %w", err)
	}
	return interaction.FollowPage{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}
