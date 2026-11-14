package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/google/uuid"
)

const dummyPassword = "password used only for constant-time login rejection"

const sessionCleanupTimeout = 2 * time.Second

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

type SessionStore interface {
	FindUserByID(context.Context, uuid.UUID) (User, error)
	ListRoleNamesForUser(context.Context, uuid.UUID) ([]string, error)
}

type RefreshSessionManager interface {
	Create(context.Context, uuid.UUID) (session.IssuedRefreshToken, error)
	Find(context.Context, string) (session.RefreshSession, error)
	Rotate(context.Context, string) (session.IssuedRefreshToken, error)
	Revoke(context.Context, string) (bool, error)
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

type SessionTokens struct {
	AccessToken  AccessToken
	RefreshToken session.IssuedRefreshToken
}

type LoginService struct {
	store             LoginStore
	password          PasswordManager
	tokens            AccessTokenIssuer
	sessions          RefreshSessionManager
	dummyPasswordHash string
}

func NewLoginService(
	store LoginStore,
	password PasswordManager,
	tokens AccessTokenIssuer,
	sessions RefreshSessionManager,
) (*LoginService, error) {
	dummyPasswordHash, err := password.Hash(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}

	return &LoginService{
		store:             store,
		password:          password,
		tokens:            tokens,
		sessions:          sessions,
		dummyPasswordHash: dummyPasswordHash,
	}, nil
}

func (s *LoginService) Login(ctx context.Context, input LoginInput) (SessionTokens, error) {
	if err := ctx.Err(); err != nil {
		return SessionTokens{}, err
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		s.consumePasswordVerification(input.Password)
		return SessionTokens{}, ErrInvalidCredentials
	}

	user, err := s.store.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			s.consumePasswordVerification(input.Password)
			return SessionTokens{}, ErrInvalidCredentials
		}

		return SessionTokens{}, fmt.Errorf("find login user: %w", err)
	}

	verified, err := s.password.Verify(input.Password, user.PasswordHash)
	if err != nil {
		return SessionTokens{}, fmt.Errorf("verify login password: %w", err)
	}
	if !verified {
		return SessionTokens{}, ErrInvalidCredentials
	}
	if user.Status != AccountStatusActive {
		return SessionTokens{}, ErrAccountUnavailable
	}

	roles, err := s.store.ListRoleNamesForUser(ctx, user.ID)
	if err != nil {
		return SessionTokens{}, fmt.Errorf("load login roles: %w", err)
	}

	issuedAt := time.Now().UTC()
	accessToken, err := s.tokens.IssueAccessToken(user.ID, roles, issuedAt)
	if err != nil {
		return SessionTokens{}, fmt.Errorf("issue access token: %w", err)
	}
	refreshToken, err := s.sessions.Create(ctx, user.ID)
	if err != nil {
		return SessionTokens{}, fmt.Errorf("create refresh session: %w", err)
	}
	if err = s.store.RecordLogin(ctx, user.ID, issuedAt); err != nil {
		revokeSessionWithoutRequest(s.sessions, refreshToken.Value)
		return SessionTokens{}, fmt.Errorf("record successful login: %w", err)
	}

	return SessionTokens{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *LoginService) consumePasswordVerification(plainText string) {
	_, _ = s.password.Verify(plainText, s.dummyPasswordHash)
}

type RefreshService struct {
	store    SessionStore
	tokens   AccessTokenIssuer
	sessions RefreshSessionManager
}

func NewRefreshService(
	store SessionStore,
	tokens AccessTokenIssuer,
	sessions RefreshSessionManager,
) *RefreshService {
	return &RefreshService{store: store, tokens: tokens, sessions: sessions}
}

func (s *RefreshService) Refresh(ctx context.Context, rawToken string) (SessionTokens, error) {
	refreshToken, err := s.sessions.Rotate(ctx, rawToken)
	if err != nil {
		return SessionTokens{}, err
	}

	user, err := s.store.FindUserByID(ctx, refreshToken.UserID)
	if err != nil {
		revokeSessionWithoutRequest(s.sessions, refreshToken.Value)
		return SessionTokens{}, fmt.Errorf("find refresh session user: %w", err)
	}
	if user.Status != AccountStatusActive {
		revokeSessionWithoutRequest(s.sessions, refreshToken.Value)
		return SessionTokens{}, ErrAccountUnavailable
	}

	roles, err := s.store.ListRoleNamesForUser(ctx, user.ID)
	if err != nil {
		revokeSessionWithoutRequest(s.sessions, refreshToken.Value)
		return SessionTokens{}, fmt.Errorf("load refresh session roles: %w", err)
	}
	accessToken, err := s.tokens.IssueAccessToken(user.ID, roles, time.Now().UTC())
	if err != nil {
		revokeSessionWithoutRequest(s.sessions, refreshToken.Value)
		return SessionTokens{}, fmt.Errorf("issue refreshed access token: %w", err)
	}

	return SessionTokens{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *RefreshService) Logout(ctx context.Context, rawToken string) (bool, error) {
	return s.sessions.Revoke(ctx, rawToken)
}

func revokeSessionWithoutRequest(sessions RefreshSessionManager, rawToken string) {
	ctx, cancel := context.WithTimeout(context.Background(), sessionCleanupTimeout)
	defer cancel()
	_, _ = sessions.Revoke(ctx, rawToken)
}
