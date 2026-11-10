package ranking

import (
	"testing"

	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
	"github.com/google/uuid"
)

func TestRankChangesWithPreferences(t *testing.T) {
	rock, jazz := uuid.New(), uuid.New()
	rockTrack, jazzTrack := uuid.New(), uuid.New()
	candidates := []Candidate{
		{TrackID: rockTrack, GenreIDs: []uuid.UUID{rock}, DiscoveryRank: 0},
		{TrackID: jazzTrack, GenreIDs: []uuid.UUID{jazz}, DiscoveryRank: 1},
	}
	items, err := Rank(preferencesignal.Set{Preferences: preferencesignal.Preferences{
		Enabled: true, Genres: []preferencesignal.WeightedPreference{{ID: jazz, Weight: 1}},
	}}, candidates, 2)
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	if items[0].TrackID != jazzTrack {
		t.Fatalf("Rank() = %+v", items)
	}

	items, err = Rank(preferencesignal.Set{Preferences: preferencesignal.Preferences{
		Enabled: true, Genres: []preferencesignal.WeightedPreference{{ID: rock, Weight: 1}},
	}}, candidates, 2)
	if err != nil || items[0].TrackID != rockTrack {
		t.Fatalf("Rank(preferred rock) = %+v, %v", items, err)
	}
}

func TestRankPenalizesExcessiveRepeats(t *testing.T) {
	genreID, recentTrack, freshTrack := uuid.New(), uuid.New(), uuid.New()
	candidates := []Candidate{{TrackID: recentTrack, GenreIDs: []uuid.UUID{genreID}}, {TrackID: freshTrack, DiscoveryRank: 1}}
	signals := preferencesignal.Set{
		Preferences: preferencesignal.Preferences{Enabled: true, Genres: []preferencesignal.WeightedPreference{{ID: genreID, Weight: 1}}},
		Listening:   []preferencesignal.Listen{{TrackID: recentTrack}, {TrackID: recentTrack}, {TrackID: recentTrack}, {TrackID: recentTrack}},
	}
	items, err := Rank(signals, candidates, 2)
	if err != nil || items[0].TrackID != freshTrack {
		t.Fatalf("Rank() = %+v, %v", items, err)
	}
}

func TestRankProvidesStableColdStartAndFiltersExplicit(t *testing.T) {
	first, second, explicit := uuid.New(), uuid.New(), uuid.New()
	candidates := []Candidate{
		{TrackID: first, DiscoveryRank: 0}, {TrackID: explicit, Explicit: true, DiscoveryRank: 1},
		{TrackID: second, DiscoveryRank: 2}, {TrackID: first, DiscoveryRank: 3},
	}
	signals := preferencesignal.Set{Preferences: preferencesignal.Preferences{Enabled: true}}
	firstRun, err := Rank(signals, candidates, 10)
	if err != nil {
		t.Fatalf("Rank() error = %v", err)
	}
	secondRun, _ := Rank(signals, candidates, 10)
	if len(firstRun) != 2 || firstRun[0].TrackID != first || firstRun[1].TrackID != second || firstRun[0].Reason != "new catalog track" {
		t.Fatalf("Rank() = %+v", firstRun)
	}
	if firstRun[0].TrackID != secondRun[0].TrackID || firstRun[1].TrackID != secondRun[1].TrackID {
		t.Fatal("cold-start order is not stable")
	}
}
