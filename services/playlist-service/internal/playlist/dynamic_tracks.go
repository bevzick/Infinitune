package playlist

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
)

type DynamicResolutionStatus string

const (
	DynamicResolutionReady DynamicResolutionStatus = "ready"
	DynamicResolutionEmpty DynamicResolutionStatus = "empty"
)

type DynamicTrack struct {
	TrackID  uuid.UUID
	Position int
}

type DynamicTrackPage struct {
	Items  []DynamicTrack
	Limit  int
	Offset int
	Total  int64
	Status DynamicResolutionStatus
}

type DynamicRuleStore interface {
	RuleSetByID(context.Context, uuid.UUID) (rules.RuleSet, error)
}

type DynamicTrackResolver interface {
	Resolve(context.Context, rules.RuleSet, PageRequest) (DynamicTrackPage, error)
}

type DynamicTrackRanker interface {
	Rank(context.Context, uuid.UUID, []uuid.UUID) ([]DynamicTrack, error)
}

type DynamicTrackService struct {
	playlists *Service
	rules     DynamicRuleStore
	resolver  DynamicTrackResolver
	ranker    DynamicTrackRanker
}

func NewDynamicTrackService(
	playlists *Service,
	rulesStore DynamicRuleStore,
	resolver DynamicTrackResolver,
	ranker DynamicTrackRanker,
) *DynamicTrackService {
	return &DynamicTrackService{
		playlists: playlists, rules: rulesStore, resolver: resolver, ranker: ranker,
	}
}

func (service *DynamicTrackService) Resolve(
	ctx context.Context,
	viewerID uuid.UUID,
	playlistID uuid.UUID,
	page PageRequest,
) (DynamicTrackPage, error) {
	current, err := service.playlists.ByID(ctx, viewerID, playlistID)
	if err != nil {
		return DynamicTrackPage{}, err
	}
	if current.Type != TypeDynamic || current.RuleSetID == nil || *current.RuleSetID == uuid.Nil {
		return DynamicTrackPage{}, ErrDynamicOnly
	}
	page, err = validateTrackPage(page)
	if err != nil {
		return DynamicTrackPage{}, err
	}
	ruleSet, err := service.rules.RuleSetByID(ctx, *current.RuleSetID)
	if err != nil {
		return DynamicTrackPage{}, fmt.Errorf("read dynamic playlist criteria: %w", err)
	}
	if ruleSet.ID != *current.RuleSetID || ruleSet.OwnerID != current.OwnerID {
		return DynamicTrackPage{}, fmt.Errorf("dynamic playlist criteria identity mismatch: %w", rules.ErrInvalid)
	}

	resolutionPage := page
	if viewerID != uuid.Nil && service.ranker != nil {
		resolutionPage = PageRequest{Limit: maximumTrackPageLimit}
	}
	result, err := service.resolver.Resolve(ctx, ruleSet, resolutionPage)
	if err != nil {
		return DynamicTrackPage{}, fmt.Errorf("resolve dynamic playlist tracks: %w", err)
	}
	if viewerID != uuid.Nil && service.ranker != nil && len(result.Items) > 0 {
		trackIDs := make([]uuid.UUID, len(result.Items))
		for index, item := range result.Items {
			trackIDs[index] = item.TrackID
		}
		ranked, rankErr := service.ranker.Rank(ctx, viewerID, trackIDs)
		if rankErr != nil {
			return DynamicTrackPage{}, fmt.Errorf("rank dynamic playlist tracks: %w", rankErr)
		}
		start := min(page.Offset, len(ranked))
		end := min(start+page.Limit, len(ranked))
		result = DynamicTrackPage{Items: append([]DynamicTrack(nil), ranked[start:end]...), Limit: page.Limit, Offset: page.Offset, Total: int64(len(ranked))}
		for index := range result.Items {
			result.Items[index].Position = start + index + 1
		}
	}
	if result.Total == 0 {
		result.Status = DynamicResolutionEmpty
	} else {
		result.Status = DynamicResolutionReady
	}
	return result, nil
}
