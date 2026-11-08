package playlist

import (
	"context"
	"errors"
	"testing"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
)

func TestDynamicTrackServiceResolvesVisiblePlaylistWithCurrentRules(t *testing.T) {
	ownerID := uuid.New()
	viewerID := uuid.New()
	playlistID := uuid.New()
	ruleSetID := uuid.New()
	trackID := uuid.New()
	base := NewService(&stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: ownerID, Type: TypeDynamic,
		Visibility: VisibilityPublic, RuleSetID: &ruleSetID,
	}})
	ruleStore := &dynamicRuleStoreStub{ruleSet: rules.RuleSet{
		ID: ruleSetID, OwnerID: ownerID, Name: "Current", Logic: rules.LogicAnd,
	}}
	resolver := &dynamicTrackResolverStub{page: DynamicTrackPage{
		Items: []DynamicTrack{{TrackID: trackID, Position: 1}}, Limit: 50, Total: 1,
	}}
	service := NewDynamicTrackService(base, ruleStore, resolver)

	result, err := service.Resolve(context.Background(), viewerID, playlistID, PageRequest{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Status != DynamicResolutionReady || result.Total != 1 ||
		len(result.Items) != 1 || result.Items[0].TrackID != trackID ||
		resolver.ruleSet.ID != ruleSetID || resolver.pageRequest.Limit != 50 {
		t.Fatalf("Resolve() = %+v; resolver = %+v", result, resolver)
	}

	ruleStore.ruleSet.Name = "Replaced"
	newTrackID := uuid.New()
	resolver.page = DynamicTrackPage{
		Items: []DynamicTrack{{TrackID: trackID}, {TrackID: newTrackID}}, Limit: 10, Total: 2,
	}
	changed, err := service.Resolve(context.Background(), viewerID, playlistID, PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Resolve() after rule replacement error = %v", err)
	}
	if resolver.ruleSet.Name != "Replaced" || resolver.calls != 2 ||
		changed.Total != 2 || changed.Items[1].TrackID != newTrackID {
		t.Fatalf("resolver did not receive current rules: %+v", resolver)
	}
}

func TestDynamicTrackServiceMarksNoMatchesAsEmpty(t *testing.T) {
	ownerID := uuid.New()
	ruleSetID := uuid.New()
	playlistID := uuid.New()
	service := NewDynamicTrackService(
		NewService(&stubStore{playlist: Playlist{
			ID: playlistID, OwnerID: ownerID, Type: TypeDynamic,
			Visibility: VisibilityPrivate, RuleSetID: &ruleSetID,
		}}),
		&dynamicRuleStoreStub{ruleSet: rules.RuleSet{ID: ruleSetID, OwnerID: ownerID}},
		&dynamicTrackResolverStub{page: DynamicTrackPage{Items: []DynamicTrack{}, Limit: 20}},
	)
	result, err := service.Resolve(
		context.Background(), ownerID, playlistID, PageRequest{Limit: 20},
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Status != DynamicResolutionEmpty || result.Items == nil || result.Total != 0 {
		t.Fatalf("Resolve() = %+v", result)
	}
}

func TestDynamicTrackServiceEnforcesVisibilityAndType(t *testing.T) {
	ownerID := uuid.New()
	ruleSetID := uuid.New()
	tests := []struct {
		name      string
		viewerID  uuid.UUID
		playlist  Playlist
		wantError error
	}{
		{name: "private non owner", viewerID: uuid.New(), playlist: Playlist{
			ID: uuid.New(), OwnerID: ownerID, Type: TypeDynamic,
			Visibility: VisibilityPrivate, RuleSetID: &ruleSetID,
		}, wantError: ErrNotFound},
		{name: "manual", viewerID: ownerID, playlist: Playlist{
			ID: uuid.New(), OwnerID: ownerID, Type: TypeManual, Visibility: VisibilityPrivate,
		}, wantError: ErrDynamicOnly},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := &dynamicTrackResolverStub{}
			service := NewDynamicTrackService(
				NewService(&stubStore{playlist: test.playlist}),
				&dynamicRuleStoreStub{}, resolver,
			)
			_, err := service.Resolve(
				context.Background(), test.viewerID, test.playlist.ID, PageRequest{},
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Resolve() error = %v, want %v", err, test.wantError)
			}
			if resolver.calls != 0 {
				t.Fatalf("resolver calls = %d, want 0", resolver.calls)
			}
		})
	}
}

func TestDynamicTrackServicePropagatesCatalogFailure(t *testing.T) {
	ownerID := uuid.New()
	ruleSetID := uuid.New()
	playlistID := uuid.New()
	resolver := &dynamicTrackResolverStub{err: ErrCatalogTimeout}
	service := NewDynamicTrackService(
		NewService(&stubStore{playlist: Playlist{
			ID: playlistID, OwnerID: ownerID, Type: TypeDynamic,
			Visibility: VisibilityPrivate, RuleSetID: &ruleSetID,
		}}),
		&dynamicRuleStoreStub{ruleSet: rules.RuleSet{ID: ruleSetID, OwnerID: ownerID}},
		resolver,
	)
	_, err := service.Resolve(context.Background(), ownerID, playlistID, PageRequest{})
	if !errors.Is(err, ErrCatalogTimeout) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrCatalogTimeout)
	}
}

type dynamicRuleStoreStub struct {
	ruleSet rules.RuleSet
	err     error
}

func (stub *dynamicRuleStoreStub) RuleSetByID(context.Context, uuid.UUID) (rules.RuleSet, error) {
	return stub.ruleSet, stub.err
}

type dynamicTrackResolverStub struct {
	page        DynamicTrackPage
	err         error
	ruleSet     rules.RuleSet
	pageRequest PageRequest
	calls       int
}

func (stub *dynamicTrackResolverStub) Resolve(
	_ context.Context,
	ruleSet rules.RuleSet,
	page PageRequest,
) (DynamicTrackPage, error) {
	stub.ruleSet = ruleSet
	stub.pageRequest = page
	stub.calls++
	return stub.page, stub.err
}
