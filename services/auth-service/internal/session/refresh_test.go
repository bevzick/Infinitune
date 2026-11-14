package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestManagerCreateFindRotateAndRevoke(t *testing.T) {
	t.Parallel()

	store := newMemoryRefreshStore()
	manager, err := NewManager(store, time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.random = bytes.NewReader(append(
		bytes.Repeat([]byte{0x11}, refreshTokenBytes),
		bytes.Repeat([]byte{0x22}, refreshTokenBytes)...,
	))
	userID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	issued, err := manager.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if issued.Value == "" || issued.UserID != userID || issued.SessionID == uuid.Nil {
		t.Fatalf("Create() = %+v", issued)
	}

	found, err := manager.Find(context.Background(), issued.Value)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if found.ID != issued.SessionID || found.UserID != userID {
		t.Fatalf("Find() = %+v", found)
	}

	rotated, err := manager.Rotate(context.Background(), issued.Value)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	if rotated.Value == issued.Value || rotated.SessionID != issued.SessionID || rotated.UserID != userID {
		t.Fatalf("Rotate() = %+v", rotated)
	}
	if _, err = manager.Find(context.Background(), issued.Value); !errors.Is(err, ErrRefreshSessionNotFound) {
		t.Fatalf("Find(old token) error = %v", err)
	}
	if _, err = manager.Find(context.Background(), rotated.Value); err != nil {
		t.Fatalf("Find(rotated token) error = %v", err)
	}

	revoked, err := manager.Revoke(context.Background(), rotated.Value)
	if err != nil || !revoked {
		t.Fatalf("Revoke() = %v, %v", revoked, err)
	}
	revoked, err = manager.Revoke(context.Background(), rotated.Value)
	if err != nil || revoked {
		t.Fatalf("Revoke(second) = %v, %v", revoked, err)
	}
}

func TestManagerRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	if _, err := NewManager(newMemoryRefreshStore(), 0); err == nil {
		t.Fatal("NewManager(0) error = nil")
	}

	manager, err := NewManager(newMemoryRefreshStore(), time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err = manager.Create(context.Background(), uuid.Nil); err == nil {
		t.Fatal("Create(nil user) error = nil")
	}
	if _, err = manager.Find(context.Background(), "invalid"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Find(invalid) error = %v", err)
	}
	if _, err = manager.Rotate(context.Background(), "invalid"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Rotate(invalid) error = %v", err)
	}
	if _, err = manager.Revoke(context.Background(), "invalid"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Revoke(invalid) error = %v", err)
	}
}

func TestManagerHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	manager, err := NewManager(newMemoryRefreshStore(), time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err = manager.Create(ctx, uuid.New()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create() error = %v", err)
	}
	validToken := base64Token(0x33)
	if _, err = manager.Rotate(ctx, validToken); !errors.Is(err, context.Canceled) {
		t.Fatalf("Rotate() error = %v", err)
	}
	if _, err = manager.Revoke(ctx, validToken); !errors.Is(err, context.Canceled) {
		t.Fatalf("Revoke() error = %v", err)
	}
}

func TestManagerStopsAfterCollisionRetryLimit(t *testing.T) {
	t.Parallel()

	store := newMemoryRefreshStore()
	store.rotateError = ErrRefreshTokenCollision
	manager, err := NewManager(store, time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.random = bytes.NewReader(bytes.Repeat(
		[]byte{0x44},
		refreshTokenBytes*maximumCollisionRetries,
	))

	_, err = manager.Rotate(context.Background(), base64Token(0x55))
	if err == nil || store.rotateCalls != maximumCollisionRetries {
		t.Fatalf("Rotate() error = %v, calls = %d", err, store.rotateCalls)
	}
}

type memoryRefreshStore struct {
	sessions    map[TokenHash]RefreshSession
	rotateError error
	rotateCalls int
}

func newMemoryRefreshStore() *memoryRefreshStore {
	return &memoryRefreshStore{sessions: make(map[TokenHash]RefreshSession)}
}

func (s *memoryRefreshStore) Save(
	_ context.Context,
	tokenHash TokenHash,
	refreshSession RefreshSession,
	_ time.Duration,
) error {
	if _, exists := s.sessions[tokenHash]; exists {
		return ErrRefreshTokenCollision
	}
	s.sessions[tokenHash] = refreshSession
	return nil
}

func (s *memoryRefreshStore) ByTokenHash(
	_ context.Context,
	tokenHash TokenHash,
) (RefreshSession, error) {
	refreshSession, exists := s.sessions[tokenHash]
	if !exists {
		return RefreshSession{}, ErrRefreshSessionNotFound
	}
	return refreshSession, nil
}

func (s *memoryRefreshStore) Rotate(
	_ context.Context,
	currentHash TokenHash,
	replacementHash TokenHash,
) (RefreshSession, error) {
	s.rotateCalls++
	if s.rotateError != nil {
		return RefreshSession{}, s.rotateError
	}
	refreshSession, exists := s.sessions[currentHash]
	if !exists {
		return RefreshSession{}, ErrRefreshSessionNotFound
	}
	if _, exists = s.sessions[replacementHash]; exists {
		return RefreshSession{}, ErrRefreshTokenCollision
	}
	delete(s.sessions, currentHash)
	s.sessions[replacementHash] = refreshSession
	return refreshSession, nil
}

func (s *memoryRefreshStore) Revoke(_ context.Context, tokenHash TokenHash) (bool, error) {
	if _, exists := s.sessions[tokenHash]; !exists {
		return false, nil
	}
	delete(s.sessions, tokenHash)
	return true, nil
}

func base64Token(value byte) string {
	secret := bytes.Repeat([]byte{value}, refreshTokenBytes)
	return base64.RawURLEncoding.EncodeToString(secret)
}
