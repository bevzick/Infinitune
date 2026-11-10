package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RunRepository struct{ pool *pgxpool.Pool }

func NewRunRepository(pool *pgxpool.Pool) *RunRepository {
	return &RunRepository{pool: pool}
}

func (repository *RunRepository) Save(
	ctx context.Context,
	run recommendation.Run,
) (recommendation.Run, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return recommendation.Run{}, fmt.Errorf("begin recommendation run transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO recommendation.recommendation_runs (
			id, user_id, algorithm_version, generated_at, expires_at
		) VALUES ($1, $2, $3, $4, $5)
	`, run.ID, run.UserID, run.AlgorithmVersion, run.GeneratedAt, run.ExpiresAt)
	if err != nil {
		if uniqueViolation(err) {
			return recommendation.Run{}, recommendation.ErrConflict
		}
		return recommendation.Run{}, fmt.Errorf("insert recommendation run: %w", err)
	}
	for _, item := range run.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO recommendation.recommendation_items (
				run_id, track_id, score, rank, reason
			) VALUES ($1, $2, $3, $4, $5)
		`, run.ID, item.TrackID, item.Score, item.Rank, item.Reason)
		if err != nil {
			return recommendation.Run{}, fmt.Errorf("insert recommendation item: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return recommendation.Run{}, fmt.Errorf("commit recommendation run: %w", err)
	}
	return cloneRun(run), nil
}

func (repository *RunRepository) ByID(
	ctx context.Context,
	runID uuid.UUID,
) (recommendation.Run, error) {
	return repository.readRun(ctx, `
		SELECT id, user_id, algorithm_version, generated_at, expires_at
		FROM recommendation.recommendation_runs
		WHERE id = $1
	`, runID)
}

func (repository *RunRepository) LatestValid(
	ctx context.Context,
	userID uuid.UUID,
	algorithmVersion string,
	at time.Time,
) (recommendation.Run, error) {
	return repository.readRun(ctx, `
		SELECT id, user_id, algorithm_version, generated_at, expires_at
		FROM recommendation.recommendation_runs
		WHERE user_id = $1
		  AND algorithm_version = $2
		  AND expires_at > $3
		ORDER BY generated_at DESC, id DESC
		LIMIT 1
	`, userID, algorithmVersion, at)
}

func (repository *RunRepository) readRun(
	ctx context.Context,
	query string,
	arguments ...any,
) (recommendation.Run, error) {
	var run recommendation.Run
	err := repository.pool.QueryRow(ctx, query, arguments...).Scan(
		&run.ID,
		&run.UserID,
		&run.AlgorithmVersion,
		&run.GeneratedAt,
		&run.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return recommendation.Run{}, recommendation.ErrNotFound
	}
	if err != nil {
		return recommendation.Run{}, fmt.Errorf("read recommendation run: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT run_id, track_id, score, rank, reason
		FROM recommendation.recommendation_items
		WHERE run_id = $1
		ORDER BY rank, track_id
	`, run.ID)
	if err != nil {
		return recommendation.Run{}, fmt.Errorf("query recommendation items: %w", err)
	}
	defer rows.Close()
	run.Items = make([]recommendation.Item, 0)
	for rows.Next() {
		var item recommendation.Item
		if err = rows.Scan(
			&item.RunID,
			&item.TrackID,
			&item.Score,
			&item.Rank,
			&item.Reason,
		); err != nil {
			return recommendation.Run{}, fmt.Errorf("scan recommendation item: %w", err)
		}
		run.Items = append(run.Items, item)
	}
	if err = rows.Err(); err != nil {
		return recommendation.Run{}, fmt.Errorf("iterate recommendation items: %w", err)
	}
	return run, nil
}

func uniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func cloneRun(run recommendation.Run) recommendation.Run {
	run.Items = append([]recommendation.Item(nil), run.Items...)
	return run
}
