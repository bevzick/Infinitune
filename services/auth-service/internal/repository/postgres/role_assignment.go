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

func (r *RoleAssignmentRepository) ListUsers(
	ctx context.Context,
	request auth.UserPageRequest,
) (auth.UserPage, error) {
	var status any
	if request.Status != nil {
		status = string(*request.Status)
	}
	filter := `($1::text IS NULL OR u.status::text = $1)`
	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM auth.users_auth AS u WHERE `+filter,
		status,
	).Scan(&total); err != nil {
		return auth.UserPage{}, fmt.Errorf("count admin users: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.password_hash, u.status, u.email_verified,
		       u.created_at, u.updated_at, u.last_login_at,
		       COALESCE(array_agg(r.name ORDER BY r.id)
		           FILTER (WHERE r.name IS NOT NULL), '{}')
		FROM auth.users_auth AS u
		LEFT JOIN auth.user_roles AS ur ON ur.user_id = u.id
		LEFT JOIN auth.roles AS r ON r.id = ur.role_id
		WHERE `+filter+`
		GROUP BY u.id
		ORDER BY u.created_at DESC, u.id
		LIMIT $2 OFFSET $3
	`, status, request.Limit, request.Offset)
	if err != nil {
		return auth.UserPage{}, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()
	items := make([]auth.AdminUser, 0, request.Limit)
	for rows.Next() {
		var item auth.AdminUser
		if err = rows.Scan(
			&item.User.ID, &item.User.Email, &item.User.PasswordHash,
			&item.User.Status, &item.User.EmailVerified, &item.User.CreatedAt,
			&item.User.UpdatedAt, &item.User.LastLoginAt, &item.Roles,
		); err != nil {
			return auth.UserPage{}, fmt.Errorf("scan admin user: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return auth.UserPage{}, fmt.Errorf("iterate admin users: %w", err)
	}
	return auth.UserPage{
		Items: items, Limit: request.Limit, Offset: request.Offset, Total: total,
	}, nil
}

func (r *RoleAssignmentRepository) UpdateUserStatus(
	ctx context.Context,
	userID uuid.UUID,
	status auth.AccountStatus,
	updatedAt time.Time,
) (auth.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin user status update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanUser(tx.QueryRow(ctx, `
		SELECT `+userColumns+` FROM auth.users_auth WHERE id = $1 FOR UPDATE
	`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("lock updated user: %w", err)
	}
	if current.Status == status {
		if err = tx.Commit(ctx); err != nil {
			return auth.User{}, fmt.Errorf("commit unchanged user status: %w", err)
		}
		return current, nil
	}
	if current.Status == auth.AccountStatusActive && status == auth.AccountStatusBlocked {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7241, 1)`); err != nil {
			return auth.User{}, fmt.Errorf("lock active admin status update: %w", err)
		}
		var targetIsAdmin bool
		if err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM auth.user_roles AS ur
				JOIN auth.roles AS role ON role.id = ur.role_id
				WHERE ur.user_id = $1 AND role.name = $2
			)
		`, userID, auth.RoleAdmin).Scan(&targetIsAdmin); err != nil {
			return auth.User{}, fmt.Errorf("check updated user admin role: %w", err)
		}
		if targetIsAdmin {
			var otherActiveAdmins int
			if err = tx.QueryRow(ctx, `
				SELECT count(*) FROM auth.users_auth AS u
				JOIN auth.user_roles AS ur ON ur.user_id = u.id
				JOIN auth.roles AS role ON role.id = ur.role_id
				WHERE role.name = $1 AND u.status = $2 AND u.id <> $3
			`, auth.RoleAdmin, auth.AccountStatusActive, userID).Scan(&otherActiveAdmins); err != nil {
				return auth.User{}, fmt.Errorf("count other active admins: %w", err)
			}
			if otherActiveAdmins == 0 {
				return auth.User{}, auth.ErrCannotDisableLastAdmin
			}
		}
	}
	updated, err := scanUser(tx.QueryRow(ctx, `
		UPDATE auth.users_auth SET status = $2, updated_at = $3
		WHERE id = $1 RETURNING `+userColumns,
		userID, status, updatedAt,
	))
	if err != nil {
		return auth.User{}, fmt.Errorf("persist user status: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return auth.User{}, fmt.Errorf("commit user status update: %w", err)
	}
	return updated, nil
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
			JOIN auth.users_auth AS u ON u.id = ur.user_id
			WHERE r.name = $1 AND u.status = $2`,
			auth.RoleAdmin, auth.AccountStatusActive,
		).Scan(&adminCount)
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
