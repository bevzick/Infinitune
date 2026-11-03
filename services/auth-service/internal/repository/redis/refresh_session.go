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
	redis_client "github.com/redis/go-redis/v9"
)

const refreshSessionKeyPrefix = "auth:refresh:"

const createRefreshSessionScriptSource = `
if redis.call("EXISTS", KEYS[1]) == 1 or redis.call("EXISTS", KEYS[2]) == 1 then
    return 0
end

redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
redis.call("SET", KEYS[2], KEYS[1], "PX", ARGV[2])
return 1
`

const rotateRefreshSessionScriptSource = `
local currentValue = redis.call("GET", KEYS[1])
if not currentValue then
    local reusedSessionID = redis.call("GET", KEYS[3])
    if reusedSessionID then
        local sessionKey = ARGV[1] .. reusedSessionID
        local activeTokenKey = redis.call("GET", sessionKey)
        if activeTokenKey then
            redis.call("DEL", activeTokenKey)
        end
        redis.call("DEL", sessionKey)
        return {-1}
    end
    return {0}
end

local remainingTTL = redis.call("PTTL", KEYS[1])
if remainingTTL <= 0 then
    return {0}
end

local decoded = cjson.decode(currentValue)
local sessionKey = ARGV[1] .. decoded.id
if redis.call("GET", sessionKey) ~= KEYS[1] then
    return {-3}
end
if redis.call("EXISTS", KEYS[2]) == 1 then
    return {-2}
end

redis.call("SET", KEYS[2], currentValue, "PX", remainingTTL)
redis.call("SET", sessionKey, KEYS[2], "PX", remainingTTL)
redis.call("DEL", KEYS[1])
redis.call("SET", KEYS[3], decoded.id, "PX", remainingTTL)
return {1, currentValue}
`

const revokeRefreshSessionScriptSource = `
local activeValue = redis.call("GET", KEYS[1])
if activeValue then
    local remainingTTL = redis.call("PTTL", KEYS[1])
    local decoded = cjson.decode(activeValue)
    local sessionKey = ARGV[1] .. decoded.id
    local currentTokenKey = redis.call("GET", sessionKey)
    if currentTokenKey then
        redis.call("DEL", currentTokenKey)
    end
    redis.call("DEL", sessionKey)
    redis.call("DEL", KEYS[1])
    if remainingTTL > 0 then
        redis.call("SET", KEYS[2], decoded.id, "PX", remainingTTL)
    end
    return 1
end

local reusedSessionID = redis.call("GET", KEYS[2])
if not reusedSessionID then
    return 0
end

local sessionKey = ARGV[1] .. reusedSessionID
local currentTokenKey = redis.call("GET", sessionKey)
if currentTokenKey then
    redis.call("DEL", currentTokenKey)
    redis.call("DEL", sessionKey)
    return 1
end

redis.call("DEL", sessionKey)
return 0
`

var (
	createRefreshSessionScript = redis_client.NewScript(createRefreshSessionScriptSource)
	rotateRefreshSessionScript = redis_client.NewScript(rotateRefreshSessionScriptSource)
	revokeRefreshSessionScript = redis_client.NewScript(revokeRefreshSessionScriptSource)
)

type refreshSessionCommands interface {
	redis_client.Scripter
	Get(context.Context, string) *redis_client.StringCmd
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

	created, err := createRefreshSessionScript.Run(
		ctx,
		r.client,
		[]string{refreshSessionKey(tokenHash), refreshSessionPointerKey(refreshSession.ID)},
		payload,
		ttl.Milliseconds(),
	).Int()
	if err != nil {
		return fmt.Errorf("store refresh session: %w", err)
	}
	if created != 1 {
		return session.ErrRefreshTokenCollision
	}

	return nil
}

func (r *RefreshSessionRepository) ByTokenHash(
	ctx context.Context,
	tokenHash session.TokenHash,
) (session.RefreshSession, error) {
	payload, err := r.client.Get(ctx, refreshSessionKey(tokenHash)).Bytes()
	if errors.Is(err, redis_client.Nil) {
		return session.RefreshSession{}, session.ErrRefreshSessionNotFound
	}
	if err != nil {
		return session.RefreshSession{}, fmt.Errorf("load refresh session: %w", err)
	}

	return decodeRefreshSession(payload)
}

func decodeRefreshSession(payload []byte) (session.RefreshSession, error) {
	var stored storedRefreshSession
	if err := json.Unmarshal(payload, &stored); err != nil {
		return session.RefreshSession{}, fmt.Errorf("decode refresh session: %w", err)
	}
	if stored.ID == uuid.Nil || stored.UserID == uuid.Nil || stored.CreatedAt.IsZero() ||
		!stored.ExpiresAt.After(stored.CreatedAt) {
		return session.RefreshSession{}, errors.New("decode refresh session: incomplete stored value")
	}

	return session.RefreshSession{
		ID:        stored.ID,
		UserID:    stored.UserID,
		CreatedAt: stored.CreatedAt,
		ExpiresAt: stored.ExpiresAt,
	}, nil
}

func (r *RefreshSessionRepository) Rotate(
	ctx context.Context,
	currentHash session.TokenHash,
	replacementHash session.TokenHash,
) (session.RefreshSession, error) {
	result, err := rotateRefreshSessionScript.Run(
		ctx,
		r.client,
		[]string{
			refreshSessionKey(currentHash),
			refreshSessionKey(replacementHash),
			usedRefreshTokenKey(currentHash),
		},
		refreshSessionPointerKeyPrefix(),
	).Slice()
	if err != nil {
		return session.RefreshSession{}, fmt.Errorf("rotate refresh session: %w", err)
	}
	if len(result) == 0 {
		return session.RefreshSession{}, errors.New("rotate refresh session: empty script result")
	}

	status, ok := result[0].(int64)
	if !ok {
		return session.RefreshSession{}, errors.New("rotate refresh session: invalid script result")
	}
	switch status {
	case 0:
		return session.RefreshSession{}, session.ErrRefreshSessionNotFound
	case -1:
		return session.RefreshSession{}, session.ErrRefreshTokenReuse
	case -2:
		return session.RefreshSession{}, session.ErrRefreshTokenCollision
	case -3:
		return session.RefreshSession{}, errors.New("rotate refresh session: inconsistent session pointer")
	case 1:
		if len(result) != 2 {
			return session.RefreshSession{}, errors.New("rotate refresh session: missing stored value")
		}
		payload, ok := result[1].(string)
		if !ok {
			return session.RefreshSession{}, errors.New("rotate refresh session: invalid stored value")
		}

		return decodeRefreshSession([]byte(payload))
	default:
		return session.RefreshSession{}, fmt.Errorf("rotate refresh session: unknown script status %d", status)
	}
}

func (r *RefreshSessionRepository) Revoke(
	ctx context.Context,
	tokenHash session.TokenHash,
) (bool, error) {
	revoked, err := revokeRefreshSessionScript.Run(
		ctx,
		r.client,
		[]string{refreshSessionKey(tokenHash), usedRefreshTokenKey(tokenHash)},
		refreshSessionPointerKeyPrefix(),
	).Int()
	if err != nil {
		return false, fmt.Errorf("revoke refresh session: %w", err)
	}
	switch revoked {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("revoke refresh session: unknown script status %d", revoked)
	}
}

func refreshSessionKey(tokenHash session.TokenHash) string {
	return refreshSessionKeyPrefix + hex.EncodeToString(tokenHash[:])
}

func usedRefreshTokenKey(tokenHash session.TokenHash) string {
	return refreshSessionKeyPrefix + "used:" + hex.EncodeToString(tokenHash[:])
}

func refreshSessionPointerKey(sessionID uuid.UUID) string {
	return refreshSessionPointerKeyPrefix() + sessionID.String()
}

func refreshSessionPointerKeyPrefix() string {
	return refreshSessionKeyPrefix + "session:"
}
