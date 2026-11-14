package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maximumEmailLength    = 320
	minimumPasswordLength = 12
	maximumPasswordLength = 128
)

var (
	ErrInvalidEmail           = errors.New("invalid email")
	ErrInvalidPassword        = errors.New("invalid password")
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInitialRoleUnavailable = errors.New("initial role unavailable")
)

type PasswordHasher interface {
	Hash(plainText string) (string, error)
}

type RegistrationStore interface {
	CreateUserWithRole(context.Context, CreateUserWithRoleParams) error
}

type CreateUserWithRoleParams struct {
	User     User
	RoleName string
}

type RegisterInput struct {
	Email    string
	Password string
}

type RegistrationResult struct {
	ID            uuid.UUID
	Email         string
	Status        AccountStatus
	EmailVerified bool
	CreatedAt     time.Time
}

type RegistrationService struct {
	store    RegistrationStore
	password PasswordHasher
}

func NewRegistrationService(store RegistrationStore, password PasswordHasher) *RegistrationService {
	return &RegistrationService{
		store:    store,
		password: password,
	}
}

func (s *RegistrationService) Register(
	ctx context.Context,
	input RegisterInput,
) (RegistrationResult, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return RegistrationResult{}, err
	}
	if err = validatePassword(input.Password); err != nil {
		return RegistrationResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return RegistrationResult{}, err
	}

	passwordHash, err := s.password.Hash(input.Password)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("hash password: %w", err)
	}

	userID, err := uuid.NewRandom()
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("generate user id: %w", err)
	}
	now := time.Now().UTC()
	user := User{
		ID:            userID,
		Email:         email,
		PasswordHash:  passwordHash,
		Status:        AccountStatusActive,
		EmailVerified: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err = s.store.CreateUserWithRole(ctx, CreateUserWithRoleParams{
		User:     user,
		RoleName: RoleListener,
	}); err != nil {
		return RegistrationResult{}, err
	}

	return RegistrationResult{
		ID:            user.ID,
		Email:         user.Email,
		Status:        user.Status,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt,
	}, nil
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || len(normalized) > maximumEmailLength || strings.ContainsAny(normalized, "\r\n") {
		return "", ErrInvalidEmail
	}

	local, domain, found := strings.Cut(normalized, "@")
	if !found || local == "" || domain == "" || strings.Contains(domain, "@") {
		return "", ErrInvalidEmail
	}

	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", ErrInvalidEmail
	}

	return normalized, nil
}

func validatePassword(value string) error {
	if !utf8.ValidString(value) {
		return ErrInvalidPassword
	}

	length := utf8.RuneCountInString(value)
	if length < minimumPasswordLength || length > maximumPasswordLength {
		return ErrInvalidPassword
	}

	return nil
}
