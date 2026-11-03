package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/jackc/pgx/v5"
)

type transactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type RegistrationRepository struct {
	db transactionBeginner
}

func NewRegistrationRepository(db transactionBeginner) *RegistrationRepository {
	return &RegistrationRepository{db: db}
}

func (r *RegistrationRepository) CreateUserWithRole(
	ctx context.Context,
	params auth.CreateUserWithRoleParams,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin user registration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	role, err := NewRoleRepository(tx).ByName(ctx, params.RoleName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return auth.ErrInitialRoleUnavailable
		}

		return fmt.Errorf("resolve initial registration role: %w", err)
	}

	user := params.User
	if _, err = NewUserRepository(tx).Create(ctx, CreateUserParams{
		ID:            user.ID,
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		Status:        user.Status,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}); err != nil {
		if errors.Is(err, ErrConflict) {
			return auth.ErrEmailAlreadyRegistered
		}

		return fmt.Errorf("persist registered user: %w", err)
	}

	if err = NewRoleRepository(tx).AssignToUser(ctx, user.ID, role.ID, user.CreatedAt); err != nil {
		return fmt.Errorf("assign initial registration role: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user registration transaction: %w", err)
	}

	return nil
}
