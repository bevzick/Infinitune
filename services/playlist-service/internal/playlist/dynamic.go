package playlist

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
)

type DynamicPlaylist struct {
	Playlist Playlist
	RuleSet  rules.RuleSet
}

type DynamicCreateInput struct {
	Playlist CreateInput
	Criteria rules.CreateInput
}

type DynamicStore interface {
	CreateDynamic(context.Context, Playlist, rules.RuleSet) (DynamicPlaylist, error)
	RuleSetByID(context.Context, uuid.UUID) (rules.RuleSet, error)
	ReplaceRuleSet(context.Context, rules.RuleSet) (rules.RuleSet, error)
}

type DynamicService struct {
	playlists *Service
	store     DynamicStore
}

func NewDynamicService(playlists *Service, store DynamicStore) *DynamicService {
	return &DynamicService{playlists: playlists, store: store}
}

func (service *DynamicService) Create(
	ctx context.Context,
	actorID uuid.UUID,
	input DynamicCreateInput,
) (DynamicPlaylist, error) {
	ruleSet, err := rules.New(actorID, input.Criteria)
	if err != nil {
		return DynamicPlaylist{}, err
	}
	value, err := newPlaylist(actorID, input.Playlist, TypeDynamic, &ruleSet.ID)
	if err != nil {
		return DynamicPlaylist{}, err
	}
	created, err := service.store.CreateDynamic(ctx, value, ruleSet)
	if err != nil {
		return DynamicPlaylist{}, fmt.Errorf("create dynamic playlist: %w", err)
	}
	return created, nil
}

func (service *DynamicService) Criteria(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
) (rules.RuleSet, error) {
	owned, err := service.dynamicOwned(ctx, actorID, playlistID)
	if err != nil {
		return rules.RuleSet{}, err
	}
	result, err := service.store.RuleSetByID(ctx, *owned.RuleSetID)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("read dynamic playlist criteria: %w", err)
	}
	if result.OwnerID != actorID {
		return rules.RuleSet{}, ErrForbidden
	}
	return result, nil
}

func (service *DynamicService) ReplaceCriteria(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
	input rules.CreateInput,
) (rules.RuleSet, error) {
	owned, err := service.dynamicOwned(ctx, actorID, playlistID)
	if err != nil {
		return rules.RuleSet{}, err
	}
	replacement, err := rules.Replace(*owned.RuleSetID, actorID, input)
	if err != nil {
		return rules.RuleSet{}, err
	}
	updated, err := service.store.ReplaceRuleSet(ctx, replacement)
	if err != nil {
		return rules.RuleSet{}, fmt.Errorf("replace dynamic playlist criteria: %w", err)
	}
	return updated, nil
}

func (service *DynamicService) dynamicOwned(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
) (Playlist, error) {
	owned, err := service.playlists.owned(ctx, actorID, playlistID)
	if err != nil {
		return Playlist{}, err
	}
	if owned.Type != TypeDynamic || owned.RuleSetID == nil || *owned.RuleSetID == uuid.Nil {
		return Playlist{}, ErrDynamicOnly
	}
	return owned, nil
}
