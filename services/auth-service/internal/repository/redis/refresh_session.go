package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
)

const refreshSessionKeyPrefix = "auth:refresh:"

type refreshSessionCommands interface {
	SetNX(context.Context, string, any, time.Duration) *redisclient.BoolCmd
	Get(context.Context, string) *redisclient.StringCmd
}

type RefreshSessionRepository struct {
	client refreshSessionCommands
}

type storedRefreshSession struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewRefreshSessionRepository(client refreshSessionCommands) *RefreshSessionRepository {
	return &RefreshSessionRepository{client: client}
}

func (r *RefreshSessionRepository) Save(
	ctx context.Context,
	tokenHash session.TokenHash,
	refreshSession session.RefreshSession,
	ttl time.Duration,
) error {
	payload, err := json.Marshal(storedRefreshSession{
		ID:        refreshSession.ID,
		UserID:    refreshSession.UserID,
		CreatedAt: refreshSession.CreatedAt,
		ExpiresAt: refreshSession.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("encode refresh session: %w", err)
	}

	created, err := r.client.SetNX(ctx, refreshSessionKey(tokenHash), payload, ttl).Result()
	if err != nil {
		return fmt.Errorf("store refresh session: %w", err)
	}
	if !created {
		return session.ErrRefreshTokenCollision
	}

	return nil
}

func (r *RefreshSessionRepository) ByTokenHash(
	ctx context.Context,
	tokenHash session.TokenHash,
) (session.RefreshSession, error) {
	payload, err := r.client.Get(ctx, refreshSessionKey(tokenHash)).Bytes()
	if errors.Is(err, redisclient.Nil) {
		return session.RefreshSession{}, session.ErrRefreshSessionNotFound
	}
	if err != nil {
		return session.RefreshSession{}, fmt.Errorf("load refresh session: %w", err)
	}

	var stored storedRefreshSession
	if err = json.Unmarshal(payload, &stored); err != nil {
		return session.RefreshSession{}, fmt.Errorf("decode refresh session: %w", err)
	}
	if stored.ID == uuid.Nil || stored.UserID == uuid.Nil || stored.CreatedAt.IsZero() || stored.ExpiresAt.IsZero() {
		return session.RefreshSession{}, errors.New("decode refresh session: incomplete stored value")
	}

	return session.RefreshSession{
		ID:        stored.ID,
		UserID:    stored.UserID,
		CreatedAt: stored.CreatedAt,
		ExpiresAt: stored.ExpiresAt,
	}, nil
}

func refreshSessionKey(tokenHash session.TokenHash) string {
	return refreshSessionKeyPrefix + hex.EncodeToString(tokenHash[:])
}
