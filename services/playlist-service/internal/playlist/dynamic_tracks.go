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

type DynamicTrackService struct {
	playlists *Service
	rules     DynamicRuleStore
	resolver  DynamicTrackResolver
}

func NewDynamicTrackService(
	playlists *Service,
	rulesStore DynamicRuleStore,
	resolver DynamicTrackResolver,
) *DynamicTrackService {
	return &DynamicTrackService{
		playlists: playlists, rules: rulesStore, resolver: resolver,
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

	result, err := service.resolver.Resolve(ctx, ruleSet, page)
	if err != nil {
		return DynamicTrackPage{}, fmt.Errorf("resolve dynamic playlist tracks: %w", err)
	}
	if result.Total == 0 {
		result.Status = DynamicResolutionEmpty
	} else {
		result.Status = DynamicResolutionReady
	}
	return result, nil
}
