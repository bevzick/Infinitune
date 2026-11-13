package adminbootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/google/uuid"
)

var (
	ErrInvalidEmail = errors.New("invalid admin email")
	ErrUserNotFound = errors.New("admin user not found")
	ErrUserInactive = errors.New("admin user is not active")
)

type Store interface {
	FindUserByEmail(context.Context, string) (auth.User, error)
	AssignAdminRole(context.Context, uuid.UUID, time.Time) error
}

type Result struct {
	UserID          uuid.UUID
	Email           string
	AlreadyAssigned bool
}

func Promote(ctx context.Context, store Store, email string) (Result, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return Result{}, err
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	user, err := store.FindUserByEmail(ctx, normalized)
	if err != nil {
		return Result{}, fmt.Errorf("find bootstrap admin: %w", err)
	}
	if user.Status != auth.AccountStatusActive {
		return Result{}, ErrUserInactive
	}
	result := Result{UserID: user.ID, Email: user.Email}
	err = store.AssignAdminRole(ctx, user.ID, time.Now().UTC())
	if errors.Is(err, auth.ErrRoleAlreadyAssigned) {
		result.AlreadyAssigned = true
		return result, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("assign bootstrap admin role: %w", err)
	}
	return result, nil
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || len(normalized) > 320 || strings.ContainsAny(normalized, "\r\n") {
		return "", ErrInvalidEmail
	}
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}
