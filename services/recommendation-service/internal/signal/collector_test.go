package signal

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestCollectorStopsWhenPersonalizationIsDisabled(t *testing.T) {
	interactions := &interactionStub{}
	history := &historyStub{}
	collector := NewCollector(preferenceStub{result: Preferences{Enabled: false}}, interactions, history, 25)
	_, err := collector.Collect(context.Background(), uuid.New())
	if !errors.Is(err, ErrPersonalizationDisabled) {
		t.Fatalf("Collect() error = %v", err)
	}
	if interactions.called || history.called {
		t.Fatal("private activity sources were called without personalization permission")
	}
}

func TestCollectorCombinesPermittedSignals(t *testing.T) {
	trackID := uuid.New()
	artistID := uuid.New()
	interactions := &interactionStub{result: Interactions{LikedTrackIDs: []uuid.UUID{trackID}, FollowedArtistIDs: []uuid.UUID{artistID}}}
	history := &historyStub{result: []Listen{{TrackID: trackID, ListenedMS: 42000, Completed: true}}}
	collector := NewCollector(preferenceStub{result: Preferences{Enabled: true}}, interactions, history, 25)
	result, err := collector.Collect(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if !interactions.called || !history.called || interactions.limit != 25 || history.limit != 25 {
		t.Fatalf("sources were not called with configured limit")
	}
	if len(result.Interactions.LikedTrackIDs) != 1 || len(result.Listening) != 1 {
		t.Fatalf("Collect() = %+v", result)
	}
}

type preferenceStub struct {
	result Preferences
	err    error
}

func (stub preferenceStub) Preferences(context.Context, uuid.UUID) (Preferences, error) {
	return stub.result, stub.err
}

type interactionStub struct {
	result Interactions
	err    error
	called bool
	limit  int
}

func (stub *interactionStub) Interactions(_ context.Context, _ uuid.UUID, limit int) (Interactions, error) {
	stub.called = true
	stub.limit = limit
	return stub.result, stub.err
}

type historyStub struct {
	result []Listen
	err    error
	called bool
	limit  int
}

func (stub *historyStub) Listening(_ context.Context, _ uuid.UUID, limit int) ([]Listen, error) {
	stub.called = true
	stub.limit = limit
	return stub.result, stub.err
}
