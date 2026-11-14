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

func (repository *CreatorRepository) ListForUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]creator.Account, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT account.id, account.account_type, account.display_name,
		       account.verification_status, account.created_at, account.updated_at
		FROM content.creator_accounts AS account
		JOIN content.creator_members AS member ON member.creator_id = account.id
		WHERE member.user_id = $1
		ORDER BY account.updated_at DESC, account.id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query creator accounts for user: %w", err)
	}
	defer rows.Close()
	accounts := make([]creator.Account, 0)
	for rows.Next() {
		account, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan creator account for user: %w", scanErr)
		}
		accounts = append(accounts, account)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate creator accounts for user: %w", err)
	}
	return accounts, nil
}

func (repository *CreatorRepository) ListVerification(
	ctx context.Context,
	reviewerID uuid.UUID,
	request creator.VerificationPageRequest,
) (creator.VerificationPage, error) {
	status := creator.VerificationPending
	if request.Status != nil {
		status = *request.Status
	}
	filter := `account.verification_status::text = $2
		AND NOT EXISTS (
			SELECT 1 FROM content.creator_members AS member
			WHERE member.creator_id = account.id AND member.user_id = $1
		)`
	var total int64
	if err := repository.database.QueryRow(ctx, `
		SELECT count(*)
		FROM content.creator_accounts AS account
		WHERE `+filter,
		reviewerID, string(status),
	).Scan(&total); err != nil {
		return creator.VerificationPage{}, fmt.Errorf("count creator verification queue: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT account.id, account.account_type, account.display_name,
		       account.verification_status, account.created_at, account.updated_at
		FROM content.creator_accounts AS account
		WHERE `+filter+`
		ORDER BY account.updated_at, account.id
		LIMIT $3 OFFSET $4
	`, reviewerID, string(status), request.Limit, request.Offset)
	if err != nil {
		return creator.VerificationPage{}, fmt.Errorf("query creator verification queue: %w", err)
	}
	defer rows.Close()
	items := make([]creator.Account, 0, request.Limit)
	for rows.Next() {
		account, scanErr := scanAccount(rows)
		if scanErr != nil {
			return creator.VerificationPage{}, fmt.Errorf("scan creator verification queue: %w", scanErr)
		}
		items = append(items, account)
	}
	if err = rows.Err(); err != nil {
		return creator.VerificationPage{}, fmt.Errorf("iterate creator verification queue: %w", err)
	}
	return creator.VerificationPage{
		Items: items, Limit: request.Limit, Offset: request.Offset, Total: total,
	}, nil
}

func (repository *CreatorRepository) Member(
	ctx context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
) (creator.Member, error) {
	member, err := scanMember(repository.database.QueryRow(ctx, `
		SELECT creator_id, user_id, member_role, created_at
		FROM content.creator_members
		WHERE creator_id = $1 AND user_id = $2
	`, creatorID, userID))
	if err != nil {
		return creator.Member{}, fmt.Errorf("find creator member: %w", mapError(err))
	}
	return member, nil
}

func (repository *CreatorRepository) ListMembers(
	ctx context.Context,
	creatorID uuid.UUID,
) ([]creator.Member, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT creator_id, user_id, member_role, created_at
		FROM content.creator_members
		WHERE creator_id = $1
		ORDER BY created_at, user_id
	`, creatorID)
	if err != nil {
		return nil, fmt.Errorf("query creator members: %w", err)
	}
	defer rows.Close()

	members := make([]creator.Member, 0)
	for rows.Next() {
		member, scanErr := scanMember(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan creator member: %w", scanErr)
		}
		members = append(members, member)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate creator members: %w", err)
	}
	return members, nil
}

func (repository *CreatorRepository) AddMember(
	ctx context.Context,
	member creator.Member,
) (creator.Member, error) {
	added, err := scanMember(repository.database.QueryRow(ctx, `
		INSERT INTO content.creator_members (creator_id, user_id, member_role)
		VALUES ($1, $2, $3)
		RETURNING creator_id, user_id, member_role, created_at
	`, member.CreatorID, member.UserID, member.Role))
	if err != nil {
		return creator.Member{}, fmt.Errorf("insert creator member: %w", mapError(err))
	}
	return added, nil
}

func (repository *CreatorRepository) UpdateMemberRole(
	ctx context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
	expected creator.MemberRole,
	target creator.MemberRole,
) (creator.Member, error) {
	updated, err := scanMember(repository.database.QueryRow(ctx, `
		UPDATE content.creator_members
		SET member_role = $4
		WHERE creator_id = $1 AND user_id = $2 AND member_role = $3
		RETURNING creator_id, user_id, member_role, created_at
	`, creatorID, userID, expected, target))
	if errors.Is(err, pgx.ErrNoRows) {
		return creator.Member{}, creator.ErrConflict
	}
	if err != nil {
		return creator.Member{}, fmt.Errorf("update creator member role: %w", mapError(err))
	}
	return updated, nil
}

func (repository *CreatorRepository) RemoveMember(
	ctx context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
	expected creator.MemberRole,
) error {
	result, err := repository.database.Exec(ctx, `
		DELETE FROM content.creator_members
		WHERE creator_id = $1 AND user_id = $2 AND member_role = $3
	`, creatorID, userID, expected)
	if err != nil {
		return fmt.Errorf("delete creator member: %w", err)
	}
	if result.RowsAffected() == 0 {
		return creator.ErrConflict
	}
	return nil
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

func scanMember(row pgx.Row) (creator.Member, error) {
	var member creator.Member
	var role string
	err := row.Scan(&member.CreatorID, &member.UserID, &role, &member.CreatedAt)
	member.Role = creator.MemberRole(role)
	return member, err
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return creator.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		if postgresError.ConstraintName == "creator_members_pkey" {
			return creator.ErrMemberExists
		}
		return creator.ErrConflict
	}
	return err
}

var (
	_ creator.Store           = (*CreatorRepository)(nil)
	_ creator.MembershipStore = (*CreatorRepository)(nil)
	_ creator.AccessStore     = (*CreatorRepository)(nil)
)
