package ranking

import (
	"context"
	"fmt"
	"slices"

	catalogv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/catalog/v1"
	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

const (
	candidatesPerFilter   = 20
	maximumFiltersPerType = 5
)

type CatalogRPC interface {
	GetTracks(context.Context, *catalogv1.GetTracksRequest, ...grpc.CallOption) (*catalogv1.GetTracksResponse, error)
	ValidateTrackIds(context.Context, *catalogv1.ValidateTrackIdsRequest, ...grpc.CallOption) (*catalogv1.ValidateTrackIdsResponse, error)
	FilterTracks(context.Context, *catalogv1.FilterTracksRequest, ...grpc.CallOption) (*catalogv1.FilterTracksResponse, error)
}

type CatalogSource struct{ client CatalogRPC }

func NewCatalogSource(client CatalogRPC) *CatalogSource { return &CatalogSource{client: client} }

func (source *CatalogSource) Candidates(ctx context.Context, trackIDs []uuid.UUID) ([]Candidate, error) {
	ids := uniqueIDStrings(trackIDs)
	if len(ids) != len(trackIDs) {
		return nil, fmt.Errorf("candidate track ids must be valid and unique")
	}
	response, err := source.client.GetTracks(ctx, &catalogv1.GetTracksRequest{TrackIds: ids})
	if err != nil {
		return nil, fmt.Errorf("read Catalog candidates: %w", err)
	}
	result, err := appendCandidates(nil, make(map[uuid.UUID]struct{}, len(ids)), response.GetTracks(), true)
	if err != nil {
		return nil, err
	}
	if len(result) != len(trackIDs) {
		return nil, fmt.Errorf("catalog returned incomplete candidate set")
	}
	return result, nil
}

func (source *CatalogSource) Discover(ctx context.Context, signals preferencesignal.Set) ([]Candidate, error) {
	var explicit *bool
	if !signals.Preferences.ExplicitAllowed {
		value := false
		explicit = &value
	}
	requests := []*catalogv1.FilterTracksRequest{{Sort: catalogv1.TrackSort_TRACK_SORT_NEWEST, Limit: 50, Explicit: explicit}}
	for _, preference := range strongestPreferences(signals.Preferences.Genres, maximumFiltersPerType) {
		requests = append(requests, &catalogv1.FilterTracksRequest{GenreId: preference.ID.String(), Sort: catalogv1.TrackSort_TRACK_SORT_NEWEST, Limit: candidatesPerFilter, Explicit: explicit})
	}
	for _, preference := range strongestPreferences(signals.Preferences.Artists, maximumFiltersPerType) {
		requests = append(requests, &catalogv1.FilterTracksRequest{ArtistId: preference.ID.String(), Sort: catalogv1.TrackSort_TRACK_SORT_NEWEST, Limit: candidatesPerFilter, Explicit: explicit})
	}
	if signals.Preferences.PreferredMoodID != nil {
		requests = append(requests, &catalogv1.FilterTracksRequest{MoodId: signals.Preferences.PreferredMoodID.String(), Sort: catalogv1.TrackSort_TRACK_SORT_NEWEST, Limit: candidatesPerFilter, Explicit: explicit})
	}
	for _, artistID := range firstIDs(signals.Interactions.FollowedArtistIDs, maximumFiltersPerType) {
		requests = append(requests, &catalogv1.FilterTracksRequest{ArtistId: artistID.String(), Sort: catalogv1.TrackSort_TRACK_SORT_NEWEST, Limit: candidatesPerFilter, Explicit: explicit})
	}
	for _, albumID := range firstIDs(signals.Interactions.LikedAlbumIDs, maximumFiltersPerType) {
		requests = append(requests, &catalogv1.FilterTracksRequest{AlbumId: albumID.String(), Sort: catalogv1.TrackSort_TRACK_SORT_NEWEST, Limit: candidatesPerFilter, Explicit: explicit})
	}

	result := make([]Candidate, 0)
	seen := make(map[uuid.UUID]struct{})
	for _, request := range requests {
		response, err := source.client.FilterTracks(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("filter Catalog candidates: %w", err)
		}
		result, err = appendCandidates(result, seen, response.GetTracks(), signals.Preferences.ExplicitAllowed)
		if err != nil {
			return nil, err
		}
	}

	directIDs := append(append([]uuid.UUID(nil), signals.Interactions.LikedTrackIDs...), listeningIDs(signals.Listening)...)
	for start := 0; start < len(directIDs); start += 100 {
		end := min(start+100, len(directIDs))
		ids := uniqueIDStrings(directIDs[start:end])
		if len(ids) == 0 {
			continue
		}
		validation, err := source.client.ValidateTrackIds(ctx, &catalogv1.ValidateTrackIdsRequest{TrackIds: ids})
		if err != nil {
			return nil, fmt.Errorf("validate direct Catalog candidates: %w", err)
		}
		if len(validation.GetValidTrackIds()) == 0 {
			continue
		}
		response, err := source.client.GetTracks(ctx, &catalogv1.GetTracksRequest{TrackIds: validation.GetValidTrackIds()})
		if err != nil {
			return nil, fmt.Errorf("read direct Catalog candidates: %w", err)
		}
		result, err = appendCandidates(result, seen, response.GetTracks(), signals.Preferences.ExplicitAllowed)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func appendCandidates(result []Candidate, seen map[uuid.UUID]struct{}, tracks []*catalogv1.Track, explicitAllowed bool) ([]Candidate, error) {
	for _, track := range tracks {
		trackID, err := uuid.Parse(track.GetId())
		if err != nil || trackID == uuid.Nil {
			return nil, fmt.Errorf("catalog returned invalid track id")
		}
		if track.GetExplicit() && !explicitAllowed {
			continue
		}
		if _, exists := seen[trackID]; exists {
			continue
		}
		candidate := Candidate{TrackID: trackID, Explicit: track.GetExplicit(), DiscoveryRank: len(result)}
		candidate.ContentID, err = uuid.Parse(track.GetContentId())
		if err != nil || candidate.ContentID == uuid.Nil {
			return nil, fmt.Errorf("catalog returned invalid content id")
		}
		if track.GetAlbumId() != "" {
			candidate.AlbumID, err = uuid.Parse(track.GetAlbumId())
			if err != nil {
				return nil, fmt.Errorf("catalog returned invalid album id")
			}
		}
		for _, credit := range track.GetArtists() {
			if credit.GetArtist() == nil {
				continue
			}
			id, parseErr := uuid.Parse(credit.GetArtist().GetId())
			if parseErr != nil {
				return nil, fmt.Errorf("catalog returned invalid artist id")
			}
			candidate.ArtistIDs = append(candidate.ArtistIDs, id)
		}
		for _, genre := range track.GetGenres() {
			id, parseErr := uuid.Parse(genre.GetId())
			if parseErr != nil {
				return nil, fmt.Errorf("catalog returned invalid genre id")
			}
			candidate.GenreIDs = append(candidate.GenreIDs, id)
		}
		for _, mood := range track.GetMoods() {
			if mood.GetMood() == nil {
				continue
			}
			id, parseErr := uuid.Parse(mood.GetMood().GetId())
			if parseErr != nil {
				return nil, fmt.Errorf("catalog returned invalid mood id")
			}
			candidate.Moods = append(candidate.Moods, Mood{ID: id, Weight: mood.GetWeight()})
		}
		seen[trackID] = struct{}{}
		result = append(result, candidate)
	}
	return result, nil
}

func strongestPreferences(values []preferencesignal.WeightedPreference, limit int) []preferencesignal.WeightedPreference {
	result := append([]preferencesignal.WeightedPreference(nil), values...)
	slices.SortFunc(result, func(left, right preferencesignal.WeightedPreference) int {
		if left.Weight != right.Weight {
			if left.Weight > right.Weight {
				return -1
			}
			return 1
		}
		return slices.Compare(left.ID[:], right.ID[:])
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}
func firstIDs(values []uuid.UUID, limit int) []uuid.UUID {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}
func listeningIDs(values []preferencesignal.Listen) []uuid.UUID {
	result := make([]uuid.UUID, len(values))
	for index, value := range values {
		result[index] = value.TrackID
	}
	return result
}
func uniqueIDStrings(values []uuid.UUID) []string {
	seen := make(map[uuid.UUID]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value.String())
	}
	return result
}
