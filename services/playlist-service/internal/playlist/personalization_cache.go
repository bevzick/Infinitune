package playlist

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

const maximumPersonalizationCacheEntries = 10_000

type PersonalizationCacheKey struct {
	PlaylistID   uuid.UUID
	ViewerID     uuid.UUID
	RuleSetID    uuid.UUID
	RulesVersion time.Time
}

type cachedPersonalization struct {
	items        []DynamicTrack
	personalized bool
	expiresAt    time.Time
}

type PersonalizationCache struct {
	mutex sync.Mutex
	ttl   time.Duration
	items map[PersonalizationCacheKey]cachedPersonalization
}

func NewPersonalizationCache(ttl time.Duration) *PersonalizationCache {
	return &PersonalizationCache{ttl: ttl, items: make(map[PersonalizationCacheKey]cachedPersonalization)}
}

func (cache *PersonalizationCache) Get(key PersonalizationCacheKey, now time.Time) ([]DynamicTrack, bool, bool) {
	if cache == nil || cache.ttl <= 0 {
		return nil, false, false
	}
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	value, exists := cache.items[key]
	if !exists {
		return nil, false, false
	}
	if !value.expiresAt.After(now) {
		delete(cache.items, key)
		return nil, false, false
	}
	return append([]DynamicTrack(nil), value.items...), value.personalized, true
}

func (cache *PersonalizationCache) Put(key PersonalizationCacheKey, items []DynamicTrack, personalized bool, now time.Time) {
	if cache == nil || cache.ttl <= 0 {
		return
	}
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if _, exists := cache.items[key]; !exists && len(cache.items) >= maximumPersonalizationCacheEntries {
		cache.evict(now)
	}
	cache.items[key] = cachedPersonalization{items: append([]DynamicTrack(nil), items...), personalized: personalized, expiresAt: now.Add(cache.ttl)}
}

func (cache *PersonalizationCache) evict(now time.Time) {
	var oldestKey PersonalizationCacheKey
	var oldestExpiry time.Time
	for key, value := range cache.items {
		if !value.expiresAt.After(now) {
			delete(cache.items, key)
			continue
		}
		if oldestExpiry.IsZero() || value.expiresAt.Before(oldestExpiry) {
			oldestKey, oldestExpiry = key, value.expiresAt
		}
	}
	if len(cache.items) >= maximumPersonalizationCacheEntries && !oldestExpiry.IsZero() {
		delete(cache.items, oldestKey)
	}
}
