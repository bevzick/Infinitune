package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDispatcherMarksPublishedAssetEvent(t *testing.T) {
	message := Message{ID: uuid.New(), Type: AssetReadyV1, Attempts: 1}
	outbox := &outboxStub{messages: []Message{message}}
	newTestDispatcher(outbox, &publisherStub{}).dispatch(context.Background())
	if outbox.publishedID != message.ID || outbox.failedID != uuid.Nil {
		t.Fatalf("published = %s, failed = %s", outbox.publishedID, outbox.failedID)
	}
}

func TestDispatcherRetriesFailedAssetEvent(t *testing.T) {
	now := time.Date(2026, time.November, 12, 12, 0, 0, 0, time.UTC)
	message := Message{ID: uuid.New(), Type: AssetReadyV1, Attempts: 2}
	outbox := &outboxStub{messages: []Message{message}}
	dispatcher := newTestDispatcher(outbox, &publisherStub{err: errors.New("unavailable")})
	dispatcher.now = func() time.Time { return now }
	dispatcher.dispatch(context.Background())
	if outbox.failedID != message.ID || !outbox.retryAt.Equal(now.Add(2*time.Second)) {
		t.Fatalf("failed = %s, retry = %v", outbox.failedID, outbox.retryAt)
	}
}

func newTestDispatcher(outbox Outbox, publisher Publisher) *Dispatcher {
	return NewDispatcher(outbox, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second, 10)
}

type outboxStub struct {
	messages              []Message
	publishedID, failedID uuid.UUID
	retryAt               time.Time
}

func (stub *outboxStub) Claim(context.Context, int, time.Time) ([]Message, error) {
	return stub.messages, nil
}
func (stub *outboxStub) MarkPublished(_ context.Context, id uuid.UUID, _ time.Time) error {
	stub.publishedID = id
	return nil
}
func (stub *outboxStub) MarkFailed(_ context.Context, id uuid.UUID, _ string, retryAt time.Time) error {
	stub.failedID = id
	stub.retryAt = retryAt
	return nil
}

type publisherStub struct{ err error }

func (stub *publisherStub) Publish(context.Context, Message) error { return stub.err }
