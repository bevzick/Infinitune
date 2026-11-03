package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/google/uuid"
)

type LoginRepository struct {
	db DBTX
}

func NewLoginRepository(db DBTX) *LoginRepository {
	return &LoginRepository{db: db}
}

func (r *LoginRepository) FindUserByEmail(ctx context.Context, email string) (auth.User, error) {
	user, err := NewUserRepository(r.db).ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return auth.User{}, auth.ErrInvalidCredentials
		}

		return auth.User{}, fmt.Errorf("find login user by email: %w", err)
	}

	return user, nil
}

func (r *LoginRepository) ListRoleNamesForUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]string, error) {
	roles, err := NewRoleRepository(r.db).ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list login user roles: %w", err)
	}

	names := make([]string, len(roles))
	for index := range roles {
		names[index] = roles[index].Name
	}

	return names, nil
}

func (r *LoginRepository) RecordLogin(ctx context.Context, userID uuid.UUID, loggedInAt time.Time) error {
	if err := NewUserRepository(r.db).RecordLogin(ctx, userID, loggedInAt); err != nil {
		return fmt.Errorf("record login time: %w", err)
	}

	return nil
}
