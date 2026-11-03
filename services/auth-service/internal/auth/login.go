package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const dummyPassword = "password used only for constant-time login rejection"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountUnavailable = errors.New("account unavailable")
)

type PasswordManager interface {
	Hash(plainText string) (string, error)
	Verify(plainText, encodedHash string) (bool, error)
}

type LoginStore interface {
	FindUserByEmail(context.Context, string) (User, error)
	ListRoleNamesForUser(context.Context, uuid.UUID) ([]string, error)
	RecordLogin(context.Context, uuid.UUID, time.Time) error
}

type AccessTokenIssuer interface {
	IssueAccessToken(uuid.UUID, []string, time.Time) (AccessToken, error)
}

type LoginInput struct {
	Email    string
	Password string
}

type AccessToken struct {
	Value     string
	Type      string
	ExpiresAt time.Time
}

type LoginService struct {
	store             LoginStore
	password          PasswordManager
	tokens            AccessTokenIssuer
	dummyPasswordHash string
}

func NewLoginService(
	store LoginStore,
	password PasswordManager,
	tokens AccessTokenIssuer,
) (*LoginService, error) {
	dummyPasswordHash, err := password.Hash(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}

	return &LoginService{
		store:             store,
		password:          password,
		tokens:            tokens,
		dummyPasswordHash: dummyPasswordHash,
	}, nil
}

func (s *LoginService) Login(ctx context.Context, input LoginInput) (AccessToken, error) {
	if err := ctx.Err(); err != nil {
		return AccessToken{}, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		s.consumePasswordVerification(input.Password)
		return AccessToken{}, ErrInvalidCredentials
	}

	user, err := s.store.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			s.consumePasswordVerification(input.Password)
			return AccessToken{}, ErrInvalidCredentials
		}

		return AccessToken{}, fmt.Errorf("find login user: %w", err)
	}

	verified, err := s.password.Verify(input.Password, user.PasswordHash)
	if err != nil {
		return AccessToken{}, fmt.Errorf("verify login password: %w", err)
	}
	if !verified {
		return AccessToken{}, ErrInvalidCredentials
	}
	if user.Status != AccountStatusActive {
		return AccessToken{}, ErrAccountUnavailable
	}

	roles, err := s.store.ListRoleNamesForUser(ctx, user.ID)
	if err != nil {
		return AccessToken{}, fmt.Errorf("load login roles: %w", err)
	}

	issuedAt := time.Now().UTC()
	accessToken, err := s.tokens.IssueAccessToken(user.ID, roles, issuedAt)
	if err != nil {
		return AccessToken{}, fmt.Errorf("issue access token: %w", err)
	}
	if err = s.store.RecordLogin(ctx, user.ID, issuedAt); err != nil {
		return AccessToken{}, fmt.Errorf("record successful login: %w", err)
	}

	return accessToken, nil
}

func (s *LoginService) consumePasswordVerification(plainText string) {
	_, _ = s.password.Verify(plainText, s.dummyPasswordHash)
}
