package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCatalogHandlerInvalidatesAllOlderRunsForPublishedRelease(t *testing.T) {
	t.Parallel()

	occurredAt := time.Date(2026, time.November, 12, 14, 0, 0, 0, time.UTC)
	invalidator := &catalogInvalidatorStub{}
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"catalog.release.published.v1",` +
		`"source":"catalog-service","occurred_at":"2026-11-12T14:00:00Z",` +
		`"data":{"source_release_id":"` + uuid.NewString() + `"}}`)
	if err := NewCatalogHandler(invalidator).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !invalidator.allBefore.Equal(occurredAt) || len(invalidator.trackIDs) != 0 {
		t.Fatalf("all before = %v, tracks = %v", invalidator.allBefore, invalidator.trackIDs)
	}
}

func TestCatalogHandlerInvalidatesRunsContainingArchivedTracks(t *testing.T) {
	t.Parallel()

	firstTrack, secondTrack := uuid.New(), uuid.New()
	invalidator := &catalogInvalidatorStub{}
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"catalog.release.archived.v1",` +
		`"source":"catalog-service","occurred_at":"2026-11-12T15:00:00Z",` +
		`"data":{"source_release_id":"` + uuid.NewString() + `","track_ids":["` +
		firstTrack.String() + `","` + secondTrack.String() + `"]}}`)
	if err := NewCatalogHandler(invalidator).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(invalidator.trackIDs) != 2 || invalidator.trackIDs[0] != firstTrack ||
		invalidator.trackIDs[1] != secondTrack || !invalidator.allBefore.IsZero() {
		t.Fatalf("tracks = %v, all before = %v", invalidator.trackIDs, invalidator.allBefore)
	}
}

func TestCatalogHandlerRejectsInvalidArchive(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"catalog.release.archived.v1",` +
		`"source":"catalog-service","occurred_at":"2026-11-12T15:00:00Z",` +
		`"data":{"source_release_id":"` + uuid.NewString() + `","track_ids":[]}}`)
	err := NewCatalogHandler(&catalogInvalidatorStub{}).Handle(context.Background(), payload)
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Handle() error = %v, want %v", err, ErrInvalidEvent)
	}
}

type catalogInvalidatorStub struct {
	allBefore time.Time
	trackIDs  []uuid.UUID
}

func (stub *catalogInvalidatorStub) InvalidateAllBefore(
	_ context.Context,
	before time.Time,
) (int64, error) {
	stub.allBefore = before
	return 1, nil
}

func (stub *catalogInvalidatorStub) InvalidateTracksBefore(
	_ context.Context,
	trackIDs []uuid.UUID,
	_ time.Time,
) (int64, error) {
	stub.trackIDs = append([]uuid.UUID(nil), trackIDs...)
	return 1, nil
}
