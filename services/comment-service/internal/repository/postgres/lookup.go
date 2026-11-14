package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (repository *CommentRepository) ExistingVisibleIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]struct{}, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT subject.id
		FROM comment.comments AS subject
		WHERE subject.id = ANY($1::uuid[])
		  AND subject.status = 'published'
		  AND (
		      subject.parent_id IS NULL OR EXISTS (
		          SELECT 1 FROM comment.comments AS parent
		          WHERE parent.id = subject.parent_id AND parent.status = 'published'
		      )
		  )
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("query visible comment ids: %w", err)
	}
	defer rows.Close()
	result := make(map[uuid.UUID]struct{}, len(ids))
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan visible comment id: %w", err)
		}
		result[id] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visible comment ids: %w", err)
	}
	return result, nil
}
