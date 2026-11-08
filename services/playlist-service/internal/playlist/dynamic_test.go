package playlist

import (
	"context"
	"errors"
	"testing"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
)

func TestDynamicServiceCreatesPlaylistAndCriteriaTogether(t *testing.T) {
	ownerID := uuid.New()
	store := &dynamicStoreStub{}
	service := NewDynamicService(NewService(&stubStore{}), store)
	created, err := service.Create(context.Background(), ownerID, DynamicCreateInput{
		Playlist: CreateInput{Name: "Fresh discoveries", Visibility: VisibilityPublic},
		Criteria: validRuleInput(),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Playlist.Type != TypeDynamic || created.Playlist.OwnerID != ownerID ||
		created.Playlist.RuleSetID == nil || *created.Playlist.RuleSetID != created.RuleSet.ID ||
		created.RuleSet.OwnerID != ownerID {
		t.Fatalf("Create() = %+v", created)
	}
	if !store.created {
		t.Fatal("Create() did not call atomic store")
	}
}

func TestDynamicServiceReadsAndReplacesOwnedCriteria(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	ruleSetID := uuid.New()
	baseStore := &stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: ownerID, Type: TypeDynamic, RuleSetID: &ruleSetID,
	}}
	store := &dynamicStoreStub{ruleSet: rules.RuleSet{ID: ruleSetID, OwnerID: ownerID}}
	service := NewDynamicService(NewService(baseStore), store)

	read, err := service.Criteria(context.Background(), ownerID, playlistID)
	if err != nil {
		t.Fatalf("Criteria() error = %v", err)
	}
	if read.ID != ruleSetID {
		t.Fatalf("Criteria().ID = %s, want %s", read.ID, ruleSetID)
	}
	updated, err := service.ReplaceCriteria(context.Background(), ownerID, playlistID, validRuleInput())
	if err != nil {
		t.Fatalf("ReplaceCriteria() error = %v", err)
	}
	if updated.ID != ruleSetID || updated.OwnerID != ownerID || len(updated.Conditions) != 1 {
		t.Fatalf("ReplaceCriteria() = %+v", updated)
	}
}

func TestDynamicServiceRejectsNonOwnerAndManualPlaylist(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	ruleSetID := uuid.New()
	tests := []struct {
		name      string
		actorID   uuid.UUID
		value     Playlist
		wantError error
	}{
		{name: "non owner", actorID: uuid.New(), value: Playlist{
			ID: playlistID, OwnerID: ownerID, Type: TypeDynamic, RuleSetID: &ruleSetID,
		}, wantError: ErrForbidden},
		{name: "manual", actorID: ownerID, value: Playlist{
			ID: playlistID, OwnerID: ownerID, Type: TypeManual,
		}, wantError: ErrDynamicOnly},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewDynamicService(NewService(&stubStore{playlist: test.value}), &dynamicStoreStub{})
			_, err := service.ReplaceCriteria(context.Background(), test.actorID, playlistID, validRuleInput())
			if !errors.Is(err, test.wantError) {
				t.Fatalf("ReplaceCriteria() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func validRuleInput() rules.CreateInput {
	return rules.CreateInput{
		Name: "Current releases", Logic: rules.LogicAnd,
		Conditions: []rules.ConditionInput{{
			Field: rules.FieldReleaseYear, Operator: rules.OperatorGreaterThanOrEqual,
			Value: rules.IntegerValue(2020),
		}},
	}
}

type dynamicStoreStub struct {
	ruleSet rules.RuleSet
	created bool
}

func (store *dynamicStoreStub) CreateDynamic(
	_ context.Context,
	value Playlist,
	ruleSet rules.RuleSet,
) (DynamicPlaylist, error) {
	store.created = true
	store.ruleSet = ruleSet
	return DynamicPlaylist{Playlist: value, RuleSet: ruleSet}, nil
}

func (store *dynamicStoreStub) RuleSetByID(context.Context, uuid.UUID) (rules.RuleSet, error) {
	return store.ruleSet, nil
}

func (store *dynamicStoreStub) ReplaceRuleSet(
	_ context.Context,
	ruleSet rules.RuleSet,
) (rules.RuleSet, error) {
	store.ruleSet = ruleSet
	return ruleSet, nil
}
