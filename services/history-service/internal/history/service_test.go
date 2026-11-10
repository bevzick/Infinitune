package history

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRecordValidatesCatalogDurationAndDerivesCompletion(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	store := &storeStub{}
	service := NewService(store, catalogStub{durationMS: 100000})
	service.now = func() time.Time { return now }
	userID, trackID, eventID := uuid.New(), uuid.New(), uuid.New()
	result, err := service.Record(context.Background(), userID, RecordInput{
		EventID: eventID, TrackID: trackID, StartedAt: now.Add(-2 * time.Minute),
		FinishedAt: now.Add(-time.Second), ListenedMS: 90000,
		ContextType: "direct", DeviceType: "web",
	})
	if err != nil || !result.Recorded || !result.Completed {
		t.Fatalf("Record() = %+v, %v", result, err)
	}
	if store.calls != 1 || store.event.UserID != userID || store.event.TrackID != trackID ||
		!store.event.Completed || store.event.ListenedMS != 90000 {
		t.Fatalf("stored event = %+v, calls = %d", store.event, store.calls)
	}
}

func TestRecordIgnoresShortPlayback(t *testing.T) {
	now := time.Now().UTC()
	store := &storeStub{}
	service := NewService(store, catalogStub{durationMS: 180000})
	service.now = func() time.Time { return now }
	result, err := service.Record(context.Background(), uuid.New(), validInput(now, 29999))
	if err != nil || result.Recorded || store.calls != 0 {
		t.Fatalf("Record(short) = %+v, %v; store calls = %d", result, err, store.calls)
	}
}

func TestRecordRejectsFakePlayback(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name     string
		input    RecordInput
		duration int
	}{
		{name: "more than duration", input: validInput(now, 180001), duration: 180000},
		{name: "more than elapsed", input: validInput(now, 60000), duration: 180000},
		{name: "stale", input: func() RecordInput {
			value := validInput(now, 30000)
			value.StartedAt = now.Add(-25 * time.Hour)
			value.FinishedAt = now.Add(-24 * time.Hour)
			return value
		}(), duration: 180000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.name == "more than elapsed" {
				test.input.StartedAt = now.Add(-10 * time.Second)
			}
			service := NewService(&storeStub{}, catalogStub{durationMS: test.duration})
			service.now = func() time.Time { return now }
			if _, err := service.Record(context.Background(), uuid.New(), test.input); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("Record() error = %v", err)
			}
		})
	}
}

func TestListIsOwnerScopedAndBuildsStableCursor(t *testing.T) {
	now := time.Now().UTC()
	userID := uuid.New()
	store := &storeStub{items: []Event{
		{ID: uuid.New(), UserID: userID, StartedAt: now},
		{ID: uuid.New(), UserID: userID, StartedAt: now.Add(-time.Minute)},
		{ID: uuid.New(), UserID: userID, StartedAt: now.Add(-2 * time.Minute)},
	}}
	service := NewService(store, catalogStub{})
	page, err := service.List(context.Background(), userID, ListRequest{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == nil || page.NextCursor.EventID != store.items[1].ID || store.userID != userID || store.limit != 3 {
		t.Fatalf("List()=%+v,%v store user/limit=%s/%d", page, err, store.userID, store.limit)
	}
}

func TestClearOnlyForVerifiedOwnerAndValidCutoff(t *testing.T) {
	now := time.Now().UTC()
	userID := uuid.New()
	before := now.Add(-time.Hour)
	store := &storeStub{deleted: 4}
	service := NewService(store, catalogStub{})
	service.now = func() time.Time { return now }
	result, err := service.Clear(context.Background(), userID, ClearRequest{Before: &before})
	if err != nil || result.Deleted != 4 || store.userID != userID || store.before == nil || !store.before.Equal(before) {
		t.Fatalf("Clear()=%+v,%v store=%+v", result, err, store)
	}
	future := now.Add(time.Hour)
	if _, err = service.Clear(context.Background(), userID, ClearRequest{Before: &future}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("Clear(future) error=%v", err)
	}
}

func validInput(now time.Time, listenedMS int) RecordInput {
	return RecordInput{EventID: uuid.New(), TrackID: uuid.New(), StartedAt: now.Add(-time.Minute),
		FinishedAt: now, ListenedMS: listenedMS, ContextType: "direct", DeviceType: "mobile"}
}

type storeStub struct {
	event   Event
	calls   int
	err     error
	items   []Event
	userID  uuid.UUID
	limit   int
	before  *time.Time
	deleted int64
}

func (stub *storeStub) List(_ context.Context, userID uuid.UUID, _ ListFilter, limit int) ([]Event, error) {
	stub.userID = userID
	stub.limit = limit
	return stub.items, stub.err
}
func (stub *storeStub) Delete(_ context.Context, userID uuid.UUID, before *time.Time) (int64, error) {
	stub.userID = userID
	stub.before = before
	return stub.deleted, stub.err
}

func (stub *storeStub) Record(_ context.Context, event Event) (Event, error) {
	stub.event = event
	stub.calls++
	return event, stub.err
}

type catalogStub struct {
	durationMS int
	err        error
}

func (stub catalogStub) Duration(context.Context, uuid.UUID) (int, error) {
	return stub.durationMS, stub.err
}
