package ranking

import (
	"bytes"
	"fmt"
	"math"
	"slices"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
	"github.com/google/uuid"
)

const (
	AlgorithmVersion  = "transparent-v1"
	defaultLimit      = 20
	maximumLimit      = 100
	genreWeight       = 3.0
	artistWeight      = 4.0
	followWeight      = 2.5
	moodWeight        = 2.0
	likedAlbumWeight  = 1.5
	likedTrackWeight  = 1.0
	repeatPenalty     = 1.0
	maximumRepeatCost = 4.0
	noveltyWeight     = 0.1
)

type scoredCandidate struct {
	candidate Candidate
	score     float64
	reason    string
}

func Rank(signals preferencesignal.Set, candidates []Candidate, limit int) ([]recommendation.Item, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maximumLimit {
		return nil, fmt.Errorf("limit must be from 1 to 100")
	}
	items := rankAll(signals, candidates)
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func rankAll(signals preferencesignal.Set, candidates []Candidate) []recommendation.Item {
	genreWeights := preferenceWeights(signals.Preferences.Genres)
	artistWeights := preferenceWeights(signals.Preferences.Artists)
	followedArtists := idSet(signals.Interactions.FollowedArtistIDs)
	likedAlbums := idSet(signals.Interactions.LikedAlbumIDs)
	likedTracks := idSet(signals.Interactions.LikedTrackIDs)
	listenCounts := make(map[uuid.UUID]int)
	for _, listen := range signals.Listening {
		listenCounts[listen.TrackID]++
	}
	coldStart := len(genreWeights) == 0 && len(artistWeights) == 0 && signals.Preferences.PreferredMoodID == nil &&
		len(followedArtists) == 0 && len(likedAlbums) == 0 && len(likedTracks) == 0 && len(listenCounts) == 0

	seen := make(map[uuid.UUID]struct{}, len(candidates))
	scored := make([]scoredCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.TrackID == uuid.Nil || candidate.DiscoveryRank < 0 {
			continue
		}
		if candidate.Explicit && !signals.Preferences.ExplicitAllowed {
			continue
		}
		if _, exists := seen[candidate.TrackID]; exists {
			continue
		}
		seen[candidate.TrackID] = struct{}{}

		score := noveltyWeight / float64(candidate.DiscoveryRank+1)
		reason, strongest := "catalog discovery", 0.0
		for _, genreID := range candidate.GenreIDs {
			if contribution := genreWeight * genreWeights[genreID]; contribution > 0 {
				score += contribution
				reason, strongest = stronger(reason, strongest, "preferred genre", contribution)
			}
		}
		for _, artistID := range candidate.ArtistIDs {
			if contribution := artistWeight * artistWeights[artistID]; contribution > 0 {
				score += contribution
				reason, strongest = stronger(reason, strongest, "preferred artist", contribution)
			}
			if _, followed := followedArtists[artistID]; followed {
				score += followWeight
				reason, strongest = stronger(reason, strongest, "followed artist", followWeight)
			}
		}
		if signals.Preferences.PreferredMoodID != nil {
			for _, mood := range candidate.Moods {
				if mood.ID == *signals.Preferences.PreferredMoodID {
					contribution := moodWeight * clamp01(mood.Weight)
					score += contribution
					reason, strongest = stronger(reason, strongest, "preferred mood", contribution)
				}
			}
		}
		if _, liked := likedAlbums[candidate.AlbumID]; liked {
			score += likedAlbumWeight
			reason, strongest = stronger(reason, strongest, "liked album", likedAlbumWeight)
		}
		if _, liked := likedTracks[candidate.TrackID]; liked {
			score += likedTrackWeight
			reason, strongest = stronger(reason, strongest, "liked track", likedTrackWeight)
		}
		if count := listenCounts[candidate.TrackID]; count > 0 {
			score -= math.Min(float64(count)*repeatPenalty, maximumRepeatCost)
		}
		if coldStart {
			reason = "new catalog track"
		}
		scored = append(scored, scoredCandidate{candidate: candidate, score: score, reason: reason})
	}

	slices.SortFunc(scored, func(left, right scoredCandidate) int {
		if left.score != right.score {
			if left.score > right.score {
				return -1
			}
			return 1
		}
		if left.candidate.DiscoveryRank != right.candidate.DiscoveryRank {
			return left.candidate.DiscoveryRank - right.candidate.DiscoveryRank
		}
		return bytes.Compare(left.candidate.TrackID[:], right.candidate.TrackID[:])
	})
	items := make([]recommendation.Item, len(scored))
	for index, value := range scored {
		items[index] = recommendation.Item{TrackID: value.candidate.TrackID, Score: value.score, Rank: index + 1, Reason: value.reason}
	}
	return items
}

func preferenceWeights(values []preferencesignal.WeightedPreference) map[uuid.UUID]float64 {
	result := make(map[uuid.UUID]float64, len(values))
	for _, value := range values {
		result[value.ID] = clamp01(value.Weight)
	}
	return result
}
func idSet(values []uuid.UUID) map[uuid.UUID]struct{} {
	result := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
func clamp01(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
func stronger(current string, currentWeight float64, candidate string, candidateWeight float64) (string, float64) {
	if candidateWeight > currentWeight {
		return candidate, candidateWeight
	}
	return current, currentWeight
}
