package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

const refreshTokenBytes = 32

var (
	ErrInvalidRefreshToken    = errors.New("invalid refresh token")
	ErrRefreshSessionNotFound = errors.New("refresh session not found")
	ErrRefreshTokenCollision  = errors.New("refresh token collision")
)

type TokenHash [sha256.Size]byte

type RefreshSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

type IssuedRefreshToken struct {
	Value     string
	SessionID uuid.UUID
	ExpiresAt time.Time
}

type Store interface {
	Save(context.Context, TokenHash, RefreshSession, time.Duration) error
	ByTokenHash(context.Context, TokenHash) (RefreshSession, error)
}

type Manager struct {
	store  Store
	ttl    time.Duration
	random io.Reader
}

func NewManager(store Store, ttl time.Duration) (*Manager, error) {
	if ttl <= 0 {
		return nil, errors.New("refresh token TTL must be positive")
	}

	return &Manager{
		store:  store,
		ttl:    ttl,
		random: rand.Reader,
	}, nil
}

func (m *Manager) Create(ctx context.Context, userID uuid.UUID) (IssuedRefreshToken, error) {
	if userID == uuid.Nil {
		return IssuedRefreshToken{}, errors.New("refresh session user id is required")
	}
	if err := ctx.Err(); err != nil {
		return IssuedRefreshToken{}, err
	}

	secret := make([]byte, refreshTokenBytes)
	if _, err := io.ReadFull(m.random, secret); err != nil {
		return IssuedRefreshToken{}, fmt.Errorf("generate refresh token: %w", err)
	}
	sessionID, err := uuid.NewRandom()
	if err != nil {
		return IssuedRefreshToken{}, fmt.Errorf("generate refresh session id: %w", err)
	}

	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(m.ttl)
	tokenHash := sha256.Sum256(secret)
	refreshSession := RefreshSession{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}
	if err = m.store.Save(ctx, tokenHash, refreshSession, m.ttl); err != nil {
		return IssuedRefreshToken{}, fmt.Errorf("persist refresh session: %w", err)
	}

	return IssuedRefreshToken{
		Value:     base64.RawURLEncoding.EncodeToString(secret),
		SessionID: sessionID,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *Manager) Find(ctx context.Context, rawToken string) (RefreshSession, error) {
	secret, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(secret) != refreshTokenBytes {
		return RefreshSession{}, ErrInvalidRefreshToken
	}

	refreshSession, err := m.store.ByTokenHash(ctx, sha256.Sum256(secret))
	if err != nil {
		return RefreshSession{}, err
	}
	if !refreshSession.ExpiresAt.After(time.Now().UTC()) {
		return RefreshSession{}, ErrRefreshSessionNotFound
	}

	return refreshSession, nil
}
