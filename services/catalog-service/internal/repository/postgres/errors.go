package postgres

import (
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/jackc/pgx/v5/pgconn"
)

func mapWriteError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return nil
	}

	switch postgresError.Code {
	case "23503":
		return fmt.Errorf("%w: %s", metadata.ErrInvalidReference, postgresError.ConstraintName)
	case "23505":
		return fmt.Errorf("%w: %s", metadata.ErrConflict, postgresError.ConstraintName)
	case "22003", "23502", "23514", "22P02":
		return fmt.Errorf("%w: %s", metadata.ErrInvalid, postgresError.ConstraintName)
	default:
		return nil
	}
}
