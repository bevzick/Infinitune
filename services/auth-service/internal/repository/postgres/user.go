package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/google/uuid"
)

const userColumns = `
	id,
	email,
	password_hash,
	status,
	email_verified,
	created_at,
	updated_at,
	last_login_at`

type CreateUserParams struct {
	ID            uuid.UUID
	Email         string
	PasswordHash  string
	Status        auth.AccountStatus
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type UserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, params CreateUserParams) (auth.User, error) {
	query := `
		INSERT INTO auth.users_auth (
			id,
			email,
			password_hash,
			status,
			email_verified,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + userColumns

	user, err := scanUser(r.db.QueryRow(
		ctx,
		query,
		params.ID,
		params.Email,
		params.PasswordHash,
		params.Status,
		params.EmailVerified,
		params.CreatedAt,
		params.UpdatedAt,
	))
	if err != nil {
		return auth.User{}, fmt.Errorf("create user: %w", mapError(err))
	}

	return user, nil
}

func (r *UserRepository) ByID(ctx context.Context, id uuid.UUID) (auth.User, error) {
	query := `SELECT ` + userColumns + ` FROM auth.users_auth WHERE id = $1`

	user, err := scanUser(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by id: %w", mapError(err))
	}

	return user, nil
}

func (r *UserRepository) ByEmail(ctx context.Context, email string) (auth.User, error) {
	query := `SELECT ` + userColumns + ` FROM auth.users_auth WHERE email = $1`

	user, err := scanUser(r.db.QueryRow(ctx, query, email))
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by email: %w", mapError(err))
	}

	return user, nil
}

func (r *UserRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status auth.AccountStatus,
	updatedAt time.Time,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE auth.users_auth SET status = $2, updated_at = $3 WHERE id = $1`,
		id,
		status,
		updatedAt,
	)
	if err != nil {
		return fmt.Errorf("update user status: %w", mapError(err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update user status: %w", ErrNotFound)
	}

	return nil
}

func (r *UserRepository) SetEmailVerified(
	ctx context.Context,
	id uuid.UUID,
	verified bool,
	updatedAt time.Time,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE auth.users_auth SET email_verified = $2, updated_at = $3 WHERE id = $1`,
		id,
		verified,
		updatedAt,
	)
	if err != nil {
		return fmt.Errorf("update user email verification: %w", mapError(err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update user email verification: %w", ErrNotFound)
	}

	return nil
}

func (r *UserRepository) RecordLogin(
	ctx context.Context,
	id uuid.UUID,
	loggedInAt time.Time,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE auth.users_auth SET last_login_at = $2, updated_at = $2 WHERE id = $1`,
		id,
		loggedInAt,
	)
	if err != nil {
		return fmt.Errorf("record user login: %w", mapError(err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("record user login: %w", ErrNotFound)
	}

	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (auth.User, error) {
	var user auth.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Status,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	return user, err
}
