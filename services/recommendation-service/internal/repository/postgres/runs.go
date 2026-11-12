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

type runDatabase interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

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
	if err = saveRun(ctx, tx, run); err != nil {
		return recommendation.Run{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return recommendation.Run{}, fmt.Errorf("commit recommendation run: %w", err)
	}
	return cloneRun(run), nil
}

// SaveIfNoValid serializes competing generations for the same user and
// algorithm. The caller may calculate a candidate optimistically; only one
// still-valid run is persisted and returned to every concurrent caller.
func (repository *RunRepository) SaveIfNoValid(
	ctx context.Context,
	candidate recommendation.Run,
	at time.Time,
) (recommendation.Run, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return recommendation.Run{}, false, fmt.Errorf("begin idempotent recommendation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	lockKey := candidate.UserID.String() + ":" + candidate.AlgorithmVersion
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return recommendation.Run{}, false, fmt.Errorf("lock recommendation generation: %w", err)
	}
	current, err := readRun(ctx, tx, `
		SELECT id, user_id, algorithm_version, generated_at, expires_at
		FROM recommendation.recommendation_runs
		WHERE user_id = $1 AND algorithm_version = $2 AND expires_at > $3
		ORDER BY generated_at DESC, id DESC
		LIMIT 1
	`, candidate.UserID, candidate.AlgorithmVersion, at)
	if err == nil {
		return current, false, nil
	}
	if !errors.Is(err, recommendation.ErrNotFound) {
		return recommendation.Run{}, false, err
	}
	if err = saveRun(ctx, tx, candidate); err != nil {
		return recommendation.Run{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return recommendation.Run{}, false, fmt.Errorf("commit idempotent recommendation run: %w", err)
	}
	return cloneRun(candidate), true, nil
}

func saveRun(ctx context.Context, database runDatabase, run recommendation.Run) error {
	_, err := database.Exec(ctx, `
		INSERT INTO recommendation.recommendation_runs (
			id, user_id, algorithm_version, generated_at, expires_at
		) VALUES ($1, $2, $3, $4, $5)
	`, run.ID, run.UserID, run.AlgorithmVersion, run.GeneratedAt, run.ExpiresAt)
	if err != nil {
		if uniqueViolation(err) {
			return recommendation.ErrConflict
		}
		return fmt.Errorf("insert recommendation run: %w", err)
	}
	for _, item := range run.Items {
		_, err = database.Exec(ctx, `
			INSERT INTO recommendation.recommendation_items (
				run_id, track_id, score, rank, reason
			) VALUES ($1, $2, $3, $4, $5)
		`, run.ID, item.TrackID, item.Score, item.Rank, item.Reason)
		if err != nil {
			return fmt.Errorf("insert recommendation item: %w", err)
		}
	}
	return nil
}

func (repository *RunRepository) ByID(
	ctx context.Context,
	runID uuid.UUID,
) (recommendation.Run, error) {
	return readRun(ctx, repository.pool, `
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
	return readRun(ctx, repository.pool, `
		SELECT id, user_id, algorithm_version, generated_at, expires_at
		FROM recommendation.recommendation_runs
		WHERE user_id = $1
		  AND algorithm_version = $2
		  AND expires_at > $3
		ORDER BY generated_at DESC, id DESC
		LIMIT 1
	`, userID, algorithmVersion, at)
}

func (repository *RunRepository) InvalidateBefore(
	ctx context.Context,
	userID uuid.UUID,
	before time.Time,
) (int64, error) {
	result, err := repository.pool.Exec(ctx, `
		UPDATE recommendation.recommendation_runs
		SET expires_at = $2
		WHERE user_id = $1
		  AND generated_at < $2
		  AND expires_at > $2
	`, userID, before)
	if err != nil {
		return 0, fmt.Errorf("invalidate recommendation runs: %w", err)
	}
	return result.RowsAffected(), nil
}

func (repository *RunRepository) InvalidateAllBefore(
	ctx context.Context,
	before time.Time,
) (int64, error) {
	result, err := repository.pool.Exec(ctx, `
		UPDATE recommendation.recommendation_runs
		SET expires_at = $1
		WHERE generated_at < $1
		  AND expires_at > $1
	`, before)
	if err != nil {
		return 0, fmt.Errorf("invalidate all recommendation runs: %w", err)
	}
	return result.RowsAffected(), nil
}

func (repository *RunRepository) InvalidateTracksBefore(
	ctx context.Context,
	trackIDs []uuid.UUID,
	before time.Time,
) (int64, error) {
	result, err := repository.pool.Exec(ctx, `
		UPDATE recommendation.recommendation_runs AS run
		SET expires_at = $2
		WHERE run.generated_at < $2
		  AND run.expires_at > $2
		  AND EXISTS (
			SELECT 1
			FROM recommendation.recommendation_items AS item
			WHERE item.run_id = run.id
			  AND item.track_id = ANY($1::uuid[])
		  )
	`, trackIDs, before)
	if err != nil {
		return 0, fmt.Errorf("invalidate recommendation runs by tracks: %w", err)
	}
	return result.RowsAffected(), nil
}

func readRun(
	ctx context.Context,
	database runDatabase,
	query string,
	arguments ...any,
) (recommendation.Run, error) {
	var run recommendation.Run
	err := database.QueryRow(ctx, query, arguments...).Scan(
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
	rows, err := database.Query(ctx, `
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
