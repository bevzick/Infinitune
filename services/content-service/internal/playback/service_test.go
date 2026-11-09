package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type storeStub struct {
	allowed bool
	err     error
	trackID uuid.UUID
	at      time.Time
}

func (stub *storeStub) IsTrackPlayable(
	_ context.Context,
	trackID uuid.UUID,
	at time.Time,
) (bool, error) {
	stub.trackID, stub.at = trackID, at
	return stub.allowed, stub.err
}

func TestServiceChecksPlaybackAtCurrentTime(t *testing.T) {
	trackID := uuid.New()
	now := time.Date(2027, time.January, 2, 3, 4, 5, 0, time.FixedZone("test", 2*60*60))
	store := &storeStub{allowed: true}
	service := NewService(store)
	service.now = func() time.Time { return now }

	allowed, err := service.IsTrackPlayable(context.Background(), trackID)
	if err != nil || !allowed || store.trackID != trackID || !store.at.Equal(now.UTC()) {
		t.Fatalf("IsTrackPlayable() = %v, %v; store = %+v", allowed, err, store)
	}
}

func TestServiceRejectsNilTrackWithoutQuery(t *testing.T) {
	store := &storeStub{allowed: true}
	allowed, err := NewService(store).IsTrackPlayable(context.Background(), uuid.Nil)
	if allowed || !errors.Is(err, ErrInvalid) || store.trackID != uuid.Nil || !store.at.IsZero() {
		t.Fatalf("IsTrackPlayable() = %v, %v; store = %+v", allowed, err, store)
	}
}

func TestServicePreservesStoreFailure(t *testing.T) {
	storeErr := errors.New("database unavailable")
	allowed, err := NewService(&storeStub{err: storeErr}).IsTrackPlayable(
		context.Background(), uuid.New(),
	)
	if allowed || !errors.Is(err, storeErr) {
		t.Fatalf("IsTrackPlayable() = %v, %v", allowed, err)
	}
}
