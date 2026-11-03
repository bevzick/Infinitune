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

const (
	refreshTokenBytes       = 32
	maximumCollisionRetries = 3
)

var (
	ErrInvalidRefreshToken    = errors.New("invalid refresh token")
	ErrRefreshSessionNotFound = errors.New("refresh session not found")
	ErrRefreshTokenCollision  = errors.New("refresh token collision")
	ErrRefreshTokenReuse      = errors.New("refresh token reuse detected")
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
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type Store interface {
	Save(context.Context, TokenHash, RefreshSession, time.Duration) error
	ByTokenHash(context.Context, TokenHash) (RefreshSession, error)
	Rotate(context.Context, TokenHash, TokenHash) (RefreshSession, error)
	Revoke(context.Context, TokenHash) (bool, error)
}

type Manager struct {
	store  Store
	ttl    time.Duration
	random io.Reader
}

func NewManager(store Store, ttl time.Duration) (*Manager, error) {
	if ttl < time.Millisecond {
		return nil, errors.New("refresh token TTL must be at least one millisecond")
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
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *Manager) Find(ctx context.Context, rawToken string) (RefreshSession, error) {
	tokenHash, err := hashRefreshToken(rawToken)
	if err != nil {
		return RefreshSession{}, ErrInvalidRefreshToken
	}

	refreshSession, err := m.store.ByTokenHash(ctx, tokenHash)
	if err != nil {
		return RefreshSession{}, err
	}
	if !refreshSession.ExpiresAt.After(time.Now().UTC()) {
		return RefreshSession{}, ErrRefreshSessionNotFound
	}

	return refreshSession, nil
}

func (m *Manager) Rotate(ctx context.Context, rawToken string) (IssuedRefreshToken, error) {
	currentHash, err := hashRefreshToken(rawToken)
	if err != nil {
		return IssuedRefreshToken{}, ErrInvalidRefreshToken
	}
	if err = ctx.Err(); err != nil {
		return IssuedRefreshToken{}, err
	}

	for range maximumCollisionRetries {
		secret := make([]byte, refreshTokenBytes)
		if _, err = io.ReadFull(m.random, secret); err != nil {
			return IssuedRefreshToken{}, fmt.Errorf("generate replacement refresh token: %w", err)
		}

		refreshSession, rotateErr := m.store.Rotate(ctx, currentHash, sha256.Sum256(secret))
		if errors.Is(rotateErr, ErrRefreshTokenCollision) {
			continue
		}
		if rotateErr != nil {
			return IssuedRefreshToken{}, fmt.Errorf("rotate refresh session: %w", rotateErr)
		}

		return IssuedRefreshToken{
			Value:     base64.RawURLEncoding.EncodeToString(secret),
			SessionID: refreshSession.ID,
			UserID:    refreshSession.UserID,
			ExpiresAt: refreshSession.ExpiresAt,
		}, nil
	}

	return IssuedRefreshToken{}, errors.New("rotate refresh session: token collision retry limit reached")
}

func (m *Manager) Revoke(ctx context.Context, rawToken string) (bool, error) {
	tokenHash, err := hashRefreshToken(rawToken)
	if err != nil {
		return false, ErrInvalidRefreshToken
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}

	revoked, err := m.store.Revoke(ctx, tokenHash)
	if err != nil {
		return false, fmt.Errorf("revoke refresh session: %w", err)
	}

	return revoked, nil
}

func hashRefreshToken(rawToken string) (TokenHash, error) {
	secret, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(secret) != refreshTokenBytes {
		return TokenHash{}, ErrInvalidRefreshToken
	}

	return sha256.Sum256(secret), nil
}
