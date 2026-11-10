package signal

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var ErrPersonalizationDisabled = errors.New("personalized recommendations are disabled")

type WeightedPreference struct {
	ID     uuid.UUID
	Weight float64
}
type Preferences struct {
	Enabled         bool
	ExplicitAllowed bool
	PreferredMoodID *uuid.UUID
	Genres          []WeightedPreference
	Artists         []WeightedPreference
}
type Interactions struct{ LikedTrackIDs, LikedAlbumIDs, FollowedArtistIDs []uuid.UUID }
type Listen struct {
	TrackID    uuid.UUID
	ListenedMS int
	Completed  bool
}
type Set struct {
	Preferences  Preferences
	Interactions Interactions
	Listening    []Listen
}

type PreferenceSource interface {
	Preferences(context.Context, uuid.UUID) (Preferences, error)
}
type InteractionSource interface {
	Interactions(context.Context, uuid.UUID, int) (Interactions, error)
}
type HistorySource interface {
	Listening(context.Context, uuid.UUID, int) ([]Listen, error)
}

type Collector struct {
	preferences  PreferenceSource
	interactions InteractionSource
	history      HistorySource
	limit        int
}

func NewCollector(preferences PreferenceSource, interactions InteractionSource, history HistorySource, limit int) *Collector {
	if limit == 0 {
		limit = 100
	}
	return &Collector{preferences: preferences, interactions: interactions, history: history, limit: limit}
}

func (collector *Collector) Collect(ctx context.Context, userID uuid.UUID) (Set, error) {
	if userID == uuid.Nil {
		return Set{}, fmt.Errorf("user id is required")
	}
	preferences, err := collector.preferences.Preferences(ctx, userID)
	if err != nil {
		return Set{}, fmt.Errorf("read user preferences: %w", err)
	}
	if !preferences.Enabled {
		return Set{}, ErrPersonalizationDisabled
	}
	interactions, err := collector.interactions.Interactions(ctx, userID, collector.limit)
	if err != nil {
		return Set{}, fmt.Errorf("read interaction signals: %w", err)
	}
	listening, err := collector.history.Listening(ctx, userID, collector.limit)
	if err != nil {
		return Set{}, fmt.Errorf("read listening signals: %w", err)
	}
	return Set{Preferences: preferences, Interactions: interactions, Listening: listening}, nil
}
