package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type roleAssignmentDB interface {
	DBTX
	Begin(context.Context) (pgx.Tx, error)
}

type RoleAssignmentRepository struct {
	db roleAssignmentDB
}

func NewRoleAssignmentRepository(db roleAssignmentDB) *RoleAssignmentRepository {
	return &RoleAssignmentRepository{db: db}
}

func (r *RoleAssignmentRepository) FindUserByID(
	ctx context.Context,
	userID uuid.UUID,
) (auth.User, error) {
	user, err := NewUserRepository(r.db).ByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return auth.User{}, auth.ErrUserNotFound
		}
		return auth.User{}, fmt.Errorf("find role assignment user: %w", err)
	}

	return user, nil
}

func (r *RoleAssignmentRepository) ListRoleNamesForUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]string, error) {
	roles, err := NewRoleRepository(r.db).ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list role assignment roles: %w", err)
	}

	names := make([]string, len(roles))
	for index := range roles {
		names[index] = roles[index].Name
	}

	return names, nil
}

func (r *RoleAssignmentRepository) AssignRoleToUser(
	ctx context.Context,
	userID uuid.UUID,
	roleName string,
	createdAt time.Time,
) error {
	role, err := NewRoleRepository(r.db).ByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return auth.ErrRoleUnavailable
		}
		return fmt.Errorf("resolve assigned role: %w", err)
	}

	err = NewRoleRepository(r.db).AssignToUser(ctx, userID, role.ID, createdAt)
	if errors.Is(err, ErrConflict) {
		return auth.ErrRoleAlreadyAssigned
	}
	if errors.Is(err, ErrInvalidReference) {
		return auth.ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("persist assigned role: %w", err)
	}

	return nil
}

func (r *RoleAssignmentRepository) RemoveRoleFromUser(
	ctx context.Context,
	userID uuid.UUID,
	roleName string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin role removal transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	role, err := NewRoleRepository(tx).ByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return auth.ErrRoleUnavailable
		}
		return fmt.Errorf("resolve removed role: %w", err)
	}

	if roleName == auth.RoleAdmin {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7241, 1)`); err != nil {
			return fmt.Errorf("lock admin role removal: %w", err)
		}
	}

	err = NewRoleRepository(tx).RemoveFromUser(ctx, userID, role.ID)
	if errors.Is(err, ErrNotFound) {
		return auth.ErrRoleNotAssigned
	}
	if err != nil {
		return fmt.Errorf("persist removed role: %w", err)
	}

	if roleName == auth.RoleAdmin {
		var adminCount int
		err = tx.QueryRow(ctx, `
			SELECT count(*)
			FROM auth.user_roles AS ur
			JOIN auth.roles AS r ON r.id = ur.role_id
			WHERE r.name = $1`, auth.RoleAdmin).Scan(&adminCount)
		if err != nil {
			return fmt.Errorf("count assigned admins: %w", err)
		}
		if adminCount == 0 {
			return auth.ErrCannotRemoveLastAdmin
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit role removal: %w", err)
	}

	return nil
}
