package playlist

import (
	"context"
	"errors"
	"testing"
	"time"

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
	service := NewDynamicTrackService(base, ruleStore, resolver, nil, nil)

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
		&dynamicTrackResolverStub{page: DynamicTrackPage{Items: []DynamicTrack{}, Limit: 20}}, nil, nil,
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
				&dynamicRuleStoreStub{}, resolver, nil, nil,
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
		resolver, nil, nil,
	)
	_, err := service.Resolve(context.Background(), ownerID, playlistID, PageRequest{})
	if !errors.Is(err, ErrCatalogTimeout) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrCatalogTimeout)
	}
}

func TestDynamicTrackServiceRanksFullCandidateWindowForViewerBeforePagination(t *testing.T) {
	ownerID, viewerID, playlistID, ruleSetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	first, second, third := uuid.New(), uuid.New(), uuid.New()
	resolver := &dynamicTrackResolverStub{page: DynamicTrackPage{Items: []DynamicTrack{{TrackID: first}, {TrackID: second}, {TrackID: third}}, Limit: 100, Total: 3}}
	ranker := &dynamicTrackRankerStub{items: []DynamicTrack{{TrackID: third}, {TrackID: first}, {TrackID: second}}, personalized: true}
	service := NewDynamicTrackService(NewService(&stubStore{playlist: Playlist{ID: playlistID, OwnerID: ownerID, Type: TypeDynamic, Visibility: VisibilityPublic, RuleSetID: &ruleSetID}}), &dynamicRuleStoreStub{ruleSet: rules.RuleSet{ID: ruleSetID, OwnerID: ownerID}}, resolver, ranker, nil)
	result, err := service.Resolve(context.Background(), viewerID, playlistID, PageRequest{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolver.pageRequest.Limit != 100 || resolver.pageRequest.Offset != 0 {
		t.Fatalf("resolver page = %+v", resolver.pageRequest)
	}
	if ranker.userID != viewerID || len(result.Items) != 2 || result.Items[0].TrackID != first || result.Items[0].Position != 2 || result.Total != 3 || !result.Personalized {
		t.Fatalf("result = %+v; ranker = %+v", result, ranker)
	}
}

func TestDynamicTrackServiceKeepsAnonymousResolutionNeutral(t *testing.T) {
	ownerID, playlistID, ruleSetID, trackID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	resolver := &dynamicTrackResolverStub{page: DynamicTrackPage{Items: []DynamicTrack{{TrackID: trackID, Position: 6}}, Limit: 1, Offset: 5, Total: 8}}
	ranker := &dynamicTrackRankerStub{personalized: true}
	service := NewDynamicTrackService(
		NewService(&stubStore{playlist: Playlist{ID: playlistID, OwnerID: ownerID, Type: TypeDynamic, Visibility: VisibilityPublic, RuleSetID: &ruleSetID}}),
		&dynamicRuleStoreStub{ruleSet: rules.RuleSet{ID: ruleSetID, OwnerID: ownerID}}, resolver, ranker, nil,
	)
	result, err := service.Resolve(context.Background(), uuid.Nil, playlistID, PageRequest{Limit: 1, Offset: 5})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if ranker.userID != uuid.Nil || resolver.pageRequest != (PageRequest{Limit: 1, Offset: 5}) || result.Personalized || result.Items[0].TrackID != trackID {
		t.Fatalf("result = %+v; resolver = %+v; ranker = %+v", result, resolver, ranker)
	}
}

func TestDynamicTrackServiceCachesPersonalizedOrderUntilRefreshOrRulesChange(t *testing.T) {
	ownerID, viewerID := uuid.New(), uuid.New()
	playlistID, ruleSetID := uuid.New(), uuid.New()
	first, second := uuid.New(), uuid.New()
	now := time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC)
	ruleStore := &dynamicRuleStoreStub{ruleSet: rules.RuleSet{
		ID: ruleSetID, OwnerID: ownerID, UpdatedAt: now.Add(-time.Hour),
	}}
	resolver := &dynamicTrackResolverStub{page: DynamicTrackPage{
		Items: []DynamicTrack{{TrackID: first}, {TrackID: second}}, Limit: 100, Total: 2,
	}}
	ranker := &dynamicTrackRankerStub{
		items: []DynamicTrack{{TrackID: second}, {TrackID: first}}, personalized: true,
	}
	service := NewDynamicTrackService(
		NewService(&stubStore{playlist: Playlist{
			ID: playlistID, OwnerID: ownerID, Type: TypeDynamic,
			Visibility: VisibilityPublic, RuleSetID: &ruleSetID,
		}}),
		ruleStore, resolver, ranker, NewPersonalizationCache(time.Hour),
	)
	service.now = func() time.Time { return now }

	firstPage, err := service.Resolve(
		context.Background(), viewerID, playlistID, PageRequest{Limit: 1},
	)
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}
	secondPage, err := service.Resolve(
		context.Background(), viewerID, playlistID, PageRequest{Limit: 1, Offset: 1},
	)
	if err != nil {
		t.Fatalf("cached Resolve() error = %v", err)
	}
	if resolver.calls != 1 || ranker.calls != 1 ||
		firstPage.Items[0].TrackID != second || secondPage.Items[0].TrackID != first ||
		secondPage.Items[0].Position != 2 {
		t.Fatalf("first = %+v; second = %+v; resolver calls = %d; ranker calls = %d",
			firstPage, secondPage, resolver.calls, ranker.calls)
	}

	ranker.items = []DynamicTrack{{TrackID: first}, {TrackID: second}}
	refreshed, err := service.Resolve(
		context.Background(), viewerID, playlistID, PageRequest{Limit: 1, Refresh: true},
	)
	if err != nil {
		t.Fatalf("refreshed Resolve() error = %v", err)
	}
	if resolver.calls != 2 || ranker.calls != 2 || refreshed.Items[0].TrackID != first {
		t.Fatalf("refreshed = %+v; resolver calls = %d; ranker calls = %d",
			refreshed, resolver.calls, ranker.calls)
	}

	ruleStore.ruleSet.UpdatedAt = now
	_, err = service.Resolve(context.Background(), viewerID, playlistID, PageRequest{Limit: 1})
	if err != nil {
		t.Fatalf("Resolve() after rules change error = %v", err)
	}
	if resolver.calls != 3 || ranker.calls != 3 {
		t.Fatalf("resolver calls = %d; ranker calls = %d, want 3", resolver.calls, ranker.calls)
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

type dynamicTrackRankerStub struct {
	items        []DynamicTrack
	personalized bool
	err          error
	userID       uuid.UUID
	calls        int
}

func (stub *dynamicTrackRankerStub) Rank(_ context.Context, userID uuid.UUID, _ []uuid.UUID) ([]DynamicTrack, bool, error) {
	stub.userID = userID
	stub.calls++
	return stub.items, stub.personalized, stub.err
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
