//go:build integration

package redis

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/database"
	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/google/uuid"
)

const integrationTimeout = 10 * time.Second

func TestRefreshSessionRepositoryLifecycle(t *testing.T) {
	client := openIntegrationRedis(t)
	repository := NewRefreshSessionRepository(client)
	currentHash := integrationTokenHash()
	replacementHash := integrationTokenHash()
	refreshSession := integrationRefreshSession()
	cleanupRefreshKeys(t, client, refreshSession.ID, currentHash, replacementHash)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	if err := repository.Save(ctx, currentHash, refreshSession, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repository.Save(ctx, currentHash, refreshSession, time.Minute); !errors.Is(err, session.ErrRefreshTokenCollision) {
		t.Fatalf("Save(duplicate) error = %v", err)
	}

	loaded, err := repository.ByTokenHash(ctx, currentHash)
	if err != nil {
		t.Fatalf("ByTokenHash() error = %v", err)
	}
	assertRefreshSessionEqual(t, loaded, refreshSession)

	rotated, err := repository.Rotate(ctx, currentHash, replacementHash)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	assertRefreshSessionEqual(t, rotated, refreshSession)
	if _, err = repository.ByTokenHash(ctx, currentHash); !errors.Is(err, session.ErrRefreshSessionNotFound) {
		t.Fatalf("ByTokenHash(old) error = %v", err)
	}
	if _, err = repository.ByTokenHash(ctx, replacementHash); err != nil {
		t.Fatalf("ByTokenHash(replacement) error = %v", err)
	}

	revoked, err := repository.Revoke(ctx, replacementHash)
	if err != nil || !revoked {
		t.Fatalf("Revoke() = %v, %v", revoked, err)
	}
	revoked, err = repository.Revoke(ctx, replacementHash)
	if err != nil || revoked {
		t.Fatalf("Revoke(second) = %v, %v", revoked, err)
	}
}

func TestRefreshSessionRepositoryDetectsConcurrentReuse(t *testing.T) {
	client := openIntegrationRedis(t)
	repository := NewRefreshSessionRepository(client)
	currentHash := integrationTokenHash()
	replacementA := integrationTokenHash()
	replacementB := integrationTokenHash()
	refreshSession := integrationRefreshSession()
	cleanupRefreshKeys(t, client, refreshSession.ID, currentHash, replacementA, replacementB)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	if err := repository.Save(ctx, currentHash, refreshSession, time.Minute); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, replacementHash := range []session.TokenHash{replacementA, replacementB} {
		replacementHash := replacementHash
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := repository.Rotate(ctx, currentHash, replacementHash)
			results <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	var successes int
	var reuseErrors int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, session.ErrRefreshTokenReuse):
			reuseErrors++
		default:
			t.Fatalf("Rotate() unexpected error = %v", err)
		}
	}
	if successes != 1 || reuseErrors != 1 {
		t.Fatalf("Rotate() successes = %d, reuse errors = %d", successes, reuseErrors)
	}

	for _, replacementHash := range []session.TokenHash{replacementA, replacementB} {
		if _, err := repository.ByTokenHash(ctx, replacementHash); !errors.Is(err, session.ErrRefreshSessionNotFound) {
			t.Fatalf("ByTokenHash(replacement) error = %v; reused family must be revoked", err)
		}
	}
}

func TestRefreshSessionRepositoryRejectsReplacementCollision(t *testing.T) {
	client := openIntegrationRedis(t)
	repository := NewRefreshSessionRepository(client)
	currentHash := integrationTokenHash()
	collidingHash := integrationTokenHash()
	currentSession := integrationRefreshSession()
	otherSession := integrationRefreshSession()
	cleanupRefreshKeys(t, client, currentSession.ID, currentHash, collidingHash)
	cleanupRefreshKeys(t, client, otherSession.ID, collidingHash)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()

	if err := repository.Save(ctx, currentHash, currentSession, time.Minute); err != nil {
		t.Fatalf("Save(current) error = %v", err)
	}
	if err := repository.Save(ctx, collidingHash, otherSession, time.Minute); err != nil {
		t.Fatalf("Save(collision) error = %v", err)
	}
	if _, err := repository.Rotate(ctx, currentHash, collidingHash); !errors.Is(err, session.ErrRefreshTokenCollision) {
		t.Fatalf("Rotate(collision) error = %v", err)
	}
	if _, err := repository.ByTokenHash(ctx, currentHash); err != nil {
		t.Fatalf("ByTokenHash(current) error = %v", err)
	}
}

func openIntegrationRedis(t *testing.T) *database.RedisClient {
	t.Helper()
	requireIntegrationEnvironment(t)

	port, err := strconv.ParseUint(requiredIntegrationEnv(t, "REDIS_PORT"), 10, 16)
	if err != nil {
		t.Fatalf("parse REDIS_PORT: %v", err)
	}
	databaseNumber, err := strconv.Atoi(requiredIntegrationEnv(t, "REDIS_AUTH_DB"))
	if err != nil {
		t.Fatalf("parse REDIS_AUTH_DB: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	client, err := database.OpenRedis(ctx, database.RedisConfig{
		Host:     requiredIntegrationEnv(t, "REDIS_HOST"),
		Port:     uint16(port),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       databaseNumber,
	})
	if err != nil {
		t.Fatalf("open integration redis: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close integration redis: %v", err)
		}
	})
	return client
}

func cleanupRefreshKeys(
	t *testing.T,
	client *database.RedisClient,
	sessionID uuid.UUID,
	tokenHashes ...session.TokenHash,
) {
	t.Helper()
	keys := []string{refreshSessionPointerKey(sessionID)}
	for _, tokenHash := range tokenHashes {
		keys = append(keys, refreshSessionKey(tokenHash), usedRefreshTokenKey(tokenHash))
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
		defer cancel()
		if err := client.Del(ctx, keys...).Err(); err != nil {
			t.Errorf("cleanup integration refresh keys: %v", err)
		}
	})
}

func integrationTokenHash() session.TokenHash {
	return sha256.Sum256([]byte(uuid.NewString()))
}

func integrationRefreshSession() session.RefreshSession {
	createdAt := time.Now().UTC().Truncate(time.Millisecond)
	return session.RefreshSession{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(time.Minute),
	}
}

func assertRefreshSessionEqual(t *testing.T, got, want session.RefreshSession) {
	t.Helper()
	if got.ID != want.ID || got.UserID != want.UserID ||
		!got.CreatedAt.Equal(want.CreatedAt) || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("refresh session = %+v, want %+v", got, want)
	}
}

func requireIntegrationEnvironment(t *testing.T) {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run repository integration tests")
	}
}

func requiredIntegrationEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required for integration tests", key)
	}
	return value
}
