package recommendation

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewRunCanonicalizesRankedItems(t *testing.T) {
	runID, userID := uuid.New(), uuid.New()
	firstTrack, secondTrack := uuid.New(), uuid.New()
	generatedAt := time.Now()
	run, err := NewRun(runID, userID, " transparent-v1 ", generatedAt,
		generatedAt.Add(time.Hour), []Item{
			{TrackID: secondTrack, Score: 0.6, Rank: 2, Reason: " recent artist "},
			{TrackID: firstTrack, Score: 0.9, Rank: 1, Reason: "preferred genre"},
		})
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	if run.AlgorithmVersion != "transparent-v1" || len(run.Items) != 2 ||
		run.Items[0].TrackID != firstTrack || run.Items[0].RunID != runID ||
		run.Items[1].Reason != "recent artist" {
		t.Fatalf("NewRun() = %+v", run)
	}
}

func TestNewRunRejectsInvalidIdentityTTLAndRanking(t *testing.T) {
	now := time.Now()
	validItem := Item{TrackID: uuid.New(), Score: 1, Rank: 1, Reason: "genre"}
	tests := []struct {
		name    string
		id      uuid.UUID
		userID  uuid.UUID
		version string
		expires time.Time
		items   []Item
	}{
		{name: "missing run", userID: uuid.New(), version: "v1", expires: now.Add(time.Hour), items: []Item{validItem}},
		{name: "missing user", id: uuid.New(), version: "v1", expires: now.Add(time.Hour), items: []Item{validItem}},
		{name: "blank version", id: uuid.New(), userID: uuid.New(), expires: now.Add(time.Hour), items: []Item{validItem}},
		{name: "expired", id: uuid.New(), userID: uuid.New(), version: "v1", expires: now, items: []Item{validItem}},
		{name: "invalid score", id: uuid.New(), userID: uuid.New(), version: "v1", expires: now.Add(time.Hour), items: []Item{{TrackID: uuid.New(), Score: math.NaN(), Rank: 1, Reason: "genre"}}},
		{name: "rank gap", id: uuid.New(), userID: uuid.New(), version: "v1", expires: now.Add(time.Hour), items: []Item{{TrackID: uuid.New(), Score: 1, Rank: 2, Reason: "genre"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRun(test.id, test.userID, test.version, now, test.expires, test.items)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("NewRun() error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}
