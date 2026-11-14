package playlist

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPersonalizationCacheCopiesValuesAndExpiresThem(t *testing.T) {
	now := time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC)
	key := PersonalizationCacheKey{
		PlaylistID: uuid.New(), ViewerID: uuid.New(), RuleSetID: uuid.New(), RulesVersion: now,
	}
	originalTrackID := uuid.New()
	items := []DynamicTrack{{TrackID: originalTrackID, Position: 1}}
	cache := NewPersonalizationCache(time.Minute)
	cache.Put(key, items, true, now)
	items[0].TrackID = uuid.New()

	cached, personalized, found := cache.Get(key, now.Add(30*time.Second))
	if !found || !personalized || cached[0].TrackID != originalTrackID {
		t.Fatalf("Get() = %+v, %t, %t", cached, personalized, found)
	}
	cached[0].TrackID = uuid.New()
	again, _, found := cache.Get(key, now.Add(30*time.Second))
	if !found || again[0].TrackID != originalTrackID {
		t.Fatalf("second Get() = %+v, found = %t", again, found)
	}
	if _, _, found = cache.Get(key, now.Add(time.Minute)); found {
		t.Fatal("Get() found expired value")
	}
}
