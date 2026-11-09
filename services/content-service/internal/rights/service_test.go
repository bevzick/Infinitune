package rights

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func TestServiceSetsNormalizedTrackRights(t *testing.T) {
	actorID, creatorID, trackID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{track: TrackTarget{
		TrackID: trackID, ReleaseID: uuid.New(), CreatorID: creatorID,
		ReleaseStatus: release.StatusDraft,
	}}
	access := &accessStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleOwner,
		VerificationStatus: creator.VerificationVerified,
	}}
	validUntil := time.Date(2028, time.January, 31, 18, 0, 0, 0, time.UTC)
	stored, err := NewService(store, access).Set(context.Background(), actorID, SetInput{
		ReleaseTrackID: trackID, TerritoryCode: " ua ", Type: TypeStreaming,
		ValidFrom: time.Date(2027, time.January, 1, 19, 30, 0, 0,
			time.FixedZone("EET", 2*60*60)),
		ValidUntil: &validUntil,
	})
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if stored.ID == uuid.Nil || stored.ReleaseTrackID != trackID ||
		stored.TerritoryCode != "UA" || stored.Type != TypeStreaming ||
		stored.GrantedByUserID != actorID || stored.ValidFrom.Location() != time.UTC ||
		stored.ValidFrom.Hour() != 0 {
		t.Fatalf("Set() = %+v", stored)
	}
	if access.actorID != actorID || access.creatorID != creatorID ||
		access.permission != creator.PermissionContentPublish {
		t.Fatalf("access check = %+v", access)
	}
}

func TestServiceRejectsInvalidRightsBeforeReadingTrack(t *testing.T) {
	trackID := uuid.New()
	validDate := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	before := validDate.AddDate(0, 0, -1)
	tests := []struct {
		name  string
		input SetInput
	}{
		{name: "missing track", input: SetInput{TerritoryCode: "UA", Type: TypeStreaming, ValidFrom: validDate}},
		{name: "invalid territory", input: SetInput{ReleaseTrackID: trackID, TerritoryCode: "UKR", Type: TypeStreaming, ValidFrom: validDate}},
		{name: "unknown territory", input: SetInput{ReleaseTrackID: trackID, TerritoryCode: "ZZ", Type: TypeStreaming, ValidFrom: validDate}},
		{name: "invalid type", input: SetInput{ReleaseTrackID: trackID, TerritoryCode: "UA", Type: "broadcast", ValidFrom: validDate}},
		{name: "missing start", input: SetInput{ReleaseTrackID: trackID, TerritoryCode: "UA", Type: TypeStreaming}},
		{name: "end before start", input: SetInput{ReleaseTrackID: trackID, TerritoryCode: TerritoryWorld, Type: TypeStreaming, ValidFrom: validDate, ValidUntil: &before}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{}
			_, err := NewService(store, &accessStub{}).Set(
				context.Background(), uuid.New(), test.input,
			)
			if !errors.Is(err, ErrInvalid) || store.trackCalls != 0 {
				t.Fatalf("Set() error = %v, track calls = %d", err, store.trackCalls)
			}
		})
	}
}

func TestServiceRejectsRightsChangesForSubmittedRelease(t *testing.T) {
	store := &storeStub{track: TrackTarget{
		TrackID: uuid.New(), CreatorID: uuid.New(),
		ReleaseStatus: release.StatusPendingReview,
	}}
	access := &accessStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleOwner,
		VerificationStatus: creator.VerificationVerified,
	}}
	_, err := NewService(store, access).Set(context.Background(), uuid.New(), SetInput{
		ReleaseTrackID: store.track.TrackID, TerritoryCode: "US",
		Type: TypeDistribution, ValidFrom: time.Now(),
	})
	if !errors.Is(err, ErrImmutable) || store.upsertCalls != 0 {
		t.Fatalf("Set() error = %v, upsert calls = %d", err, store.upsertCalls)
	}
}

func TestServiceRequiresVerifiedPublishingMember(t *testing.T) {
	trackID := uuid.New()
	store := &storeStub{track: TrackTarget{
		TrackID: trackID, CreatorID: uuid.New(), ReleaseStatus: release.StatusDraft,
	}}
	access := &accessStub{decision: creator.AccessDecision{
		Role: creator.RoleManager, VerificationStatus: creator.VerificationUnverified,
	}}
	_, err := NewService(store, access).Set(context.Background(), uuid.New(), SetInput{
		ReleaseTrackID: trackID, TerritoryCode: TerritoryWorld,
		Type: TypeDistribution, ValidFrom: time.Now(),
	})
	if !errors.Is(err, creator.ErrVerificationRequired) || store.upsertCalls != 0 {
		t.Fatalf("Set() error = %v, upsert calls = %d", err, store.upsertCalls)
	}
}

func TestServiceListsRightsForCreatorMember(t *testing.T) {
	actorID, creatorID, trackID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{
		track:  TrackTarget{TrackID: trackID, CreatorID: creatorID},
		rights: []Right{{ID: uuid.New(), ReleaseTrackID: trackID}},
	}
	access := &accessStub{decision: creator.AccessDecision{Allowed: true}}
	result, err := NewService(store, access).List(context.Background(), actorID, trackID)
	if err != nil || len(result) != 1 || access.permission != creator.PermissionAccountRead {
		t.Fatalf("List() = %+v, %v; permission = %q", result, err, access.permission)
	}
}

func TestServiceDeletesMutableTrackRights(t *testing.T) {
	actorID, creatorID, rightID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{right: RightTarget{
		Right: Right{ID: rightID},
		Track: TrackTarget{
			CreatorID: creatorID, ReleaseStatus: release.StatusRejected,
		},
	}}
	access := &accessStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleManager,
		VerificationStatus: creator.VerificationVerified,
	}}
	if err := NewService(store, access).Delete(
		context.Background(), actorID, rightID,
	); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.deletedID != rightID || access.permission != creator.PermissionContentPublish {
		t.Fatalf("deleted ID = %s, permission = %q", store.deletedID, access.permission)
	}
}

type storeStub struct {
	track       TrackTarget
	right       RightTarget
	rights      []Right
	trackCalls  int
	upsertCalls int
	deletedID   uuid.UUID
}

func (stub *storeStub) TrackTarget(context.Context, uuid.UUID) (TrackTarget, error) {
	stub.trackCalls++
	return stub.track, nil
}

func (stub *storeStub) RightTarget(context.Context, uuid.UUID) (RightTarget, error) {
	return stub.right, nil
}

func (stub *storeStub) UpsertRight(_ context.Context, right Right) (Right, error) {
	stub.upsertCalls++
	stub.rights = []Right{right}
	return right, nil
}

func (stub *storeStub) ListRights(context.Context, uuid.UUID) ([]Right, error) {
	return stub.rights, nil
}

func (stub *storeStub) DeleteRight(_ context.Context, rightID uuid.UUID) error {
	stub.deletedID = rightID
	return nil
}

type accessStub struct {
	decision   creator.AccessDecision
	err        error
	actorID    uuid.UUID
	creatorID  uuid.UUID
	permission creator.Permission
}

func (stub *accessStub) Check(
	_ context.Context,
	actorID uuid.UUID,
	creatorID uuid.UUID,
	permission creator.Permission,
) (creator.AccessDecision, error) {
	stub.actorID, stub.creatorID, stub.permission = actorID, creatorID, permission
	return stub.decision, stub.err
}

var _ Store = (*storeStub)(nil)
var _ AccessChecker = (*accessStub)(nil)
