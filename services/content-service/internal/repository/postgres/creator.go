package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const accountColumns = `
	id, account_type, display_name, verification_status, created_at, updated_at
`

type CreatorRepository struct {
	database *pgxpool.Pool
}

func NewCreatorRepository(database *pgxpool.Pool) *CreatorRepository {
	return &CreatorRepository{database: database}
}

func (repository *CreatorRepository) Create(
	ctx context.Context,
	account creator.Account,
	owner creator.Member,
) (creator.Account, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return creator.Account{}, fmt.Errorf("begin creator account transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	created, err := scanAccount(tx.QueryRow(ctx, `
		INSERT INTO content.creator_accounts (
			id, account_type, display_name, verification_status
		)
		VALUES ($1, $2, $3, $4)
		RETURNING `+accountColumns,
		account.ID, account.Type, account.DisplayName, account.VerificationStatus,
	))
	if err != nil {
		return creator.Account{}, fmt.Errorf("insert creator account: %w", mapError(err))
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO content.creator_members (creator_id, user_id, member_role)
		VALUES ($1, $2, $3)
	`, owner.CreatorID, owner.UserID, owner.Role); err != nil {
		return creator.Account{}, fmt.Errorf("insert creator account owner: %w", mapError(err))
	}
	if err = tx.Commit(ctx); err != nil {
		return creator.Account{}, fmt.Errorf("commit creator account transaction: %w", err)
	}
	return created, nil
}

func (repository *CreatorRepository) ByID(
	ctx context.Context,
	id uuid.UUID,
) (creator.Account, error) {
	account, err := scanAccount(repository.database.QueryRow(ctx, `
		SELECT `+accountColumns+`
		FROM content.creator_accounts
		WHERE id = $1
	`, id))
	if err != nil {
		return creator.Account{}, fmt.Errorf("find creator account: %w", mapError(err))
	}
	return account, nil
}

func (repository *CreatorRepository) Member(
	ctx context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
) (creator.Member, error) {
	var member creator.Member
	var role string
	err := repository.database.QueryRow(ctx, `
		SELECT creator_id, user_id, member_role, created_at
		FROM content.creator_members
		WHERE creator_id = $1 AND user_id = $2
	`, creatorID, userID).Scan(
		&member.CreatorID, &member.UserID, &role, &member.CreatedAt,
	)
	if err != nil {
		return creator.Member{}, fmt.Errorf("find creator member: %w", mapError(err))
	}
	member.Role = creator.MemberRole(role)
	return member, nil
}

func (repository *CreatorRepository) Update(
	ctx context.Context,
	account creator.Account,
	expectedUpdatedAt time.Time,
) (creator.Account, error) {
	updated, err := scanAccount(repository.database.QueryRow(ctx, `
		UPDATE content.creator_accounts
		SET account_type = $2,
		    display_name = $3,
		    verification_status = $4,
		    updated_at = clock_timestamp()
		WHERE id = $1 AND updated_at = $5
		RETURNING `+accountColumns,
		account.ID, account.Type, account.DisplayName, account.VerificationStatus,
		expectedUpdatedAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return creator.Account{}, creator.ErrConflict
	}
	if err != nil {
		return creator.Account{}, fmt.Errorf("update creator account: %w", mapError(err))
	}
	return updated, nil
}

func (repository *CreatorRepository) TransitionVerification(
	ctx context.Context,
	id uuid.UUID,
	expected creator.VerificationStatus,
	target creator.VerificationStatus,
) (creator.Account, error) {
	updated, err := scanAccount(repository.database.QueryRow(ctx, `
		UPDATE content.creator_accounts
		SET verification_status = $3, updated_at = clock_timestamp()
		WHERE id = $1 AND verification_status = $2
		RETURNING `+accountColumns,
		id, expected, target,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return creator.Account{}, creator.ErrConflict
	}
	if err != nil {
		return creator.Account{}, fmt.Errorf("transition creator verification: %w", mapError(err))
	}
	return updated, nil
}

func scanAccount(row pgx.Row) (creator.Account, error) {
	var account creator.Account
	var accountType string
	var status string
	err := row.Scan(
		&account.ID,
		&accountType,
		&account.DisplayName,
		&status,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	account.Type = creator.AccountType(accountType)
	account.VerificationStatus = creator.VerificationStatus(status)
	return account, err
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return creator.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return creator.ErrConflict
	}
	return err
}

var _ creator.Store = (*CreatorRepository)(nil)
