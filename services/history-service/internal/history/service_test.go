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

func validInput(now time.Time, listenedMS int) RecordInput {
	return RecordInput{EventID: uuid.New(), TrackID: uuid.New(), StartedAt: now.Add(-time.Minute),
		FinishedAt: now, ListenedMS: listenedMS, ContextType: "direct", DeviceType: "mobile"}
}

type storeStub struct {
	event Event
	calls int
	err   error
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
