package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound         = errors.New("record not found")
	ErrConflict         = errors.New("record already exists")
	ErrInvalidReference = errors.New("referenced record does not exist")
)

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Code {
	case "23505":
		return ErrConflict
	case "23503":
		return ErrInvalidReference
	default:
		return err
	}
}
