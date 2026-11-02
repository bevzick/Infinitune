package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const roleColumns = `id, name, description`

type CreateRoleParams struct {
	Name        string
	Description *string
}

type RoleRepository struct {
	db DBTX
}

func NewRoleRepository(db DBTX) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(ctx context.Context, params CreateRoleParams) (auth.Role, error) {
	query := `
		INSERT INTO auth.roles (name, description)
		VALUES ($1, $2)
		RETURNING ` + roleColumns

	role, err := scanRole(r.db.QueryRow(ctx, query, params.Name, params.Description))
	if err != nil {
		return auth.Role{}, fmt.Errorf("create role: %w", mapError(err))
	}

	return role, nil
}

func (r *RoleRepository) ByID(ctx context.Context, id int16) (auth.Role, error) {
	query := `SELECT ` + roleColumns + ` FROM auth.roles WHERE id = $1`

	role, err := scanRole(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return auth.Role{}, fmt.Errorf("find role by id: %w", mapError(err))
	}

	return role, nil
}

func (r *RoleRepository) ByName(ctx context.Context, name string) (auth.Role, error) {
	query := `SELECT ` + roleColumns + ` FROM auth.roles WHERE name = $1`

	role, err := scanRole(r.db.QueryRow(ctx, query, name))
	if err != nil {
		return auth.Role{}, fmt.Errorf("find role by name: %w", mapError(err))
	}

	return role, nil
}

func (r *RoleRepository) List(ctx context.Context) ([]auth.Role, error) {
	query := `SELECT ` + roleColumns + ` FROM auth.roles ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", mapError(err))
	}
	defer rows.Close()

	roles, err := pgx.CollectRows(rows, pgx.RowToStructByPos[auth.Role])
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", mapError(err))
	}

	return roles, nil
}

func (r *RoleRepository) AssignToUser(
	ctx context.Context,
	userID uuid.UUID,
	roleID int16,
	createdAt time.Time,
) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO auth.user_roles (user_id, role_id, created_at) VALUES ($1, $2, $3)`,
		userID,
		roleID,
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("assign role to user: %w", mapError(err))
	}

	return nil
}

func (r *RoleRepository) RemoveFromUser(
	ctx context.Context,
	userID uuid.UUID,
	roleID int16,
) error {
	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM auth.user_roles WHERE user_id = $1 AND role_id = $2`,
		userID,
		roleID,
	)
	if err != nil {
		return fmt.Errorf("remove role from user: %w", mapError(err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("remove role from user: %w", ErrNotFound)
	}

	return nil
}

func (r *RoleRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]auth.Role, error) {
	query := `
		SELECT r.id, r.name, r.description
		FROM auth.roles AS r
		JOIN auth.user_roles AS ur ON ur.role_id = r.id
		WHERE ur.user_id = $1
		ORDER BY r.id`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list roles for user: %w", mapError(err))
	}
	defer rows.Close()

	roles, err := pgx.CollectRows(rows, pgx.RowToStructByPos[auth.Role])
	if err != nil {
		return nil, fmt.Errorf("list roles for user: %w", mapError(err))
	}

	return roles, nil
}

func scanRole(row rowScanner) (auth.Role, error) {
	var role auth.Role
	err := row.Scan(&role.ID, &role.Name, &role.Description)

	return role, err
}
