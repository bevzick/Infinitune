package ranking

import (
	"context"
	"testing"

	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
	"github.com/google/uuid"
)

func TestServiceCollectsDiscoversAndRanks(t *testing.T) {
	userID, trackID := uuid.New(), uuid.New()
	contentID := uuid.New()
	signals := preferencesignal.Set{Preferences: preferencesignal.Preferences{Enabled: true}}
	service := NewService(signalCollectorStub{result: signals}, candidateSourceStub{result: []Candidate{{TrackID: trackID, ContentID: contentID}}}, eligibilityStub{allowed: true})
	items, err := service.Rank(context.Background(), userID, 10)
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	if len(items) != 1 || items[0].TrackID != trackID || items[0].Rank != 1 {
		t.Fatalf("Rank() = %+v", items)
	}
}

func TestServiceSkipsTracksWithoutPlaybackRights(t *testing.T) {
	deniedTrack, allowedTrack := uuid.New(), uuid.New()
	deniedContent, allowedContent := uuid.New(), uuid.New()
	service := NewService(
		signalCollectorStub{result: preferencesignal.Set{Preferences: preferencesignal.Preferences{Enabled: true}}},
		candidateSourceStub{result: []Candidate{{TrackID: deniedTrack, ContentID: deniedContent}, {TrackID: allowedTrack, ContentID: allowedContent, DiscoveryRank: 1}}},
		eligibilityStub{allowedIDs: map[uuid.UUID]bool{allowedContent: true}},
	)
	items, err := service.Rank(context.Background(), uuid.New(), 1)
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	if len(items) != 1 || items[0].TrackID != allowedTrack || items[0].Rank != 1 {
		t.Fatalf("Rank() = %+v", items)
	}
}

func TestServiceBackfillsBeyondFirstHundredDeniedCandidates(t *testing.T) {
	candidates := make([]Candidate, 101)
	allowedContent := uuid.New()
	allowedTrack := uuid.New()
	allowedIDs := map[uuid.UUID]bool{allowedContent: true}
	for index := range candidates {
		candidates[index] = Candidate{
			TrackID: uuid.New(), ContentID: uuid.New(), DiscoveryRank: index,
		}
	}
	candidates[100] = Candidate{
		TrackID: allowedTrack, ContentID: allowedContent, DiscoveryRank: 100,
	}
	service := NewService(
		signalCollectorStub{result: preferencesignal.Set{Preferences: preferencesignal.Preferences{Enabled: true}}},
		candidateSourceStub{result: candidates},
		eligibilityStub{allowedIDs: allowedIDs},
	)
	items, err := service.Rank(context.Background(), uuid.New(), 1)
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	if len(items) != 1 || items[0].TrackID != allowedTrack {
		t.Fatalf("Rank() = %+v", items)
	}
}

type signalCollectorStub struct {
	result preferencesignal.Set
	err    error
}

func (stub signalCollectorStub) Collect(context.Context, uuid.UUID) (preferencesignal.Set, error) {
	return stub.result, stub.err
}

type candidateSourceStub struct {
	result []Candidate
	err    error
}

func (stub candidateSourceStub) Discover(context.Context, preferencesignal.Set) ([]Candidate, error) {
	return stub.result, stub.err
}
func (stub candidateSourceStub) Candidates(context.Context, []uuid.UUID) ([]Candidate, error) {
	return stub.result, stub.err
}

type eligibilityStub struct {
	allowed    bool
	allowedIDs map[uuid.UUID]bool
}

func (stub eligibilityStub) Allowed(_ context.Context, id uuid.UUID) (bool, error) {
	if stub.allowedIDs != nil {
		return stub.allowedIDs[id], nil
	}
	return stub.allowed, nil
}
