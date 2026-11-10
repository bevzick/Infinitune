package recommendation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceReusesValidRunWithoutRanking(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	cached := validRun(t, uuid.New(), now)
	ranker := &rankerStub{}
	store := &runStoreStub{latest: cached}
	service := newTestService(t, ranker, store, now)
	result, err := service.Get(context.Background(), cached.UserID, false)
	if err != nil || result.ID != cached.ID {
		t.Fatalf("Get() = %+v, %v", result, err)
	}
	if ranker.calls != 0 || store.saveIfCalls != 0 {
		t.Fatal("valid cache triggered regeneration")
	}
}

func TestServiceGeneratesRunWithTTLOnCacheMiss(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	userID, trackID, runID := uuid.New(), uuid.New(), uuid.New()
	ranker := &rankerStub{items: []Item{{TrackID: trackID, Score: 2.5, Rank: 1, Reason: "preferred artist"}}}
	store := &runStoreStub{latestErr: ErrNotFound}
	service := newTestService(t, ranker, store, now)
	service.newID = func() uuid.UUID { return runID }
	result, err := service.Get(context.Background(), userID, false)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.ID != runID || result.AlgorithmVersion != "transparent-v1" || !result.ExpiresAt.Equal(now.Add(time.Hour)) || result.Items[0].RunID != runID {
		t.Fatalf("Get() = %+v", result)
	}
	if ranker.limit != generatedItemLimit || store.saveIfCalls != 1 {
		t.Fatalf("rank/store calls = %d/%d", ranker.calls, store.saveIfCalls)
	}
}

func TestServiceReturnsWinnerOfConcurrentGeneration(t *testing.T) {
	now := time.Now().UTC()
	userID := uuid.New()
	winner := validRun(t, userID, now)
	store := &runStoreStub{latestErr: ErrNotFound, saveIfResult: winner}
	service := newTestService(t, &rankerStub{}, store, now)
	result, err := service.Get(context.Background(), userID, false)
	if err != nil || result.ID != winner.ID {
		t.Fatalf("Get() = %+v, %v", result, err)
	}
}

func TestServiceRefreshBypassesValidCache(t *testing.T) {
	now := time.Now().UTC()
	userID := uuid.New()
	store := &runStoreStub{latest: validRun(t, userID, now)}
	ranker := &rankerStub{}
	service := newTestService(t, ranker, store, now)
	result, err := service.Get(context.Background(), userID, true)
	if err != nil {
		t.Fatalf("Get(refresh) error = %v", err)
	}
	if result.ID == store.latest.ID || ranker.calls != 1 || store.saveCalls != 1 || store.latestCalls != 0 {
		t.Fatalf("Get(refresh) = %+v; store = %+v", result, store)
	}
}

func newTestService(t *testing.T, ranker *rankerStub, store *runStoreStub, now time.Time) *Service {
	t.Helper()
	service, err := NewService(ranker, store, "transparent-v1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	return service
}

func validRun(t *testing.T, userID uuid.UUID, now time.Time) Run {
	t.Helper()
	run, err := NewRun(uuid.New(), userID, "transparent-v1", now, now.Add(time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

type rankerStub struct {
	items        []Item
	err          error
	calls, limit int
}

func (stub *rankerStub) Rank(_ context.Context, _ uuid.UUID, limit int) ([]Item, error) {
	stub.calls++
	stub.limit = limit
	return stub.items, stub.err
}

type runStoreStub struct {
	latest                              Run
	latestErr                           error
	saveIfResult                        Run
	latestCalls, saveCalls, saveIfCalls int
}

func (stub *runStoreStub) LatestValid(context.Context, uuid.UUID, string, time.Time) (Run, error) {
	stub.latestCalls++
	return stub.latest, stub.latestErr
}
func (stub *runStoreStub) Save(_ context.Context, run Run) (Run, error) {
	stub.saveCalls++
	return run, nil
}
func (stub *runStoreStub) SaveIfNoValid(_ context.Context, candidate Run, _ time.Time) (Run, bool, error) {
	stub.saveIfCalls++
	if stub.saveIfResult.ID != uuid.Nil {
		return stub.saveIfResult, false, nil
	}
	return candidate, true, nil
}
