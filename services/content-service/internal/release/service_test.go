package release

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
)

func TestServiceCreatesSingleDraft(t *testing.T) {
	creatorID, actorID := uuid.New(), uuid.New()
	store := &storeStub{}
	authorizer := &authorizerStub{}
	service := NewService(store, authorizer)

	draft, err := service.CreateDraft(context.Background(), actorID, CreateDraftInput{
		CreatorID: creatorID, Title: "  First Light  ", Type: TypeSingle,
		ReleaseDate: time.Date(2027, time.March, 12, 18, 30, 0, 0, time.FixedZone("EET", 2*60*60)),
		Tracks:      []TrackInput{{Title: " First Light ", DiscNumber: 1, TrackNumber: 1}},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if draft.Release.ID == uuid.Nil || draft.Release.CreatorID != creatorID ||
		draft.Release.Title != "First Light" || draft.Release.Type != TypeSingle ||
		draft.Release.Status != StatusDraft || len(draft.Tracks) != 1 {
		t.Fatalf("CreateDraft() = %+v", draft)
	}
	if draft.Release.ReleaseDate.Location() != time.UTC ||
		draft.Release.ReleaseDate.Hour() != 0 || draft.Release.ReleaseDate.Day() != 12 {
		t.Fatalf("ReleaseDate = %v", draft.Release.ReleaseDate)
	}
	if draft.Tracks[0].ID == uuid.Nil || draft.Tracks[0].ReleaseID != draft.Release.ID {
		t.Fatalf("track = %+v", draft.Tracks[0])
	}
	if authorizer.actorID != actorID || authorizer.creatorID != creatorID ||
		authorizer.permission != creator.PermissionContentEdit {
		t.Fatalf("authorization = %+v", authorizer)
	}
}

func TestServiceCreatesMultiTrackAlbumInDeclaredOrder(t *testing.T) {
	draft, err := NewService(&storeStub{}, &authorizerStub{}).CreateDraft(
		context.Background(), uuid.New(), CreateDraftInput{
			CreatorID: uuid.New(), Title: "Night Transit", Type: TypeAlbum,
			ReleaseDate: time.Date(2027, time.June, 1, 0, 0, 0, 0, time.UTC),
			Tracks: []TrackInput{
				{Title: "Departure", DiscNumber: 1, TrackNumber: 1},
				{Title: "Arrival", DiscNumber: 1, TrackNumber: 2, Explicit: true},
			},
		},
	)
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if len(draft.Tracks) != 2 || draft.Tracks[0].TrackNumber != 1 ||
		draft.Tracks[1].TrackNumber != 2 || !draft.Tracks[1].Explicit {
		t.Fatalf("tracks = %+v", draft.Tracks)
	}
}

func TestServiceRejectsInvalidDraftsBeforeAuthorization(t *testing.T) {
	validDate := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		input CreateDraftInput
	}{
		{name: "missing creator", input: CreateDraftInput{Title: "Title", Type: TypeSingle, ReleaseDate: validDate, Tracks: []TrackInput{{Title: "Track", DiscNumber: 1, TrackNumber: 1}}}},
		{name: "blank title", input: CreateDraftInput{CreatorID: uuid.New(), Title: " ", Type: TypeSingle, ReleaseDate: validDate, Tracks: []TrackInput{{Title: "Track", DiscNumber: 1, TrackNumber: 1}}}},
		{name: "unsupported type", input: CreateDraftInput{CreatorID: uuid.New(), Title: "Title", Type: "mixtape", ReleaseDate: validDate, Tracks: []TrackInput{{Title: "Track", DiscNumber: 1, TrackNumber: 1}}}},
		{name: "empty track list", input: CreateDraftInput{CreatorID: uuid.New(), Title: "Title", Type: TypeAlbum, ReleaseDate: validDate}},
		{name: "multiple single tracks", input: CreateDraftInput{CreatorID: uuid.New(), Title: "Title", Type: TypeSingle, ReleaseDate: validDate, Tracks: []TrackInput{{Title: "One", DiscNumber: 1, TrackNumber: 1}, {Title: "Two", DiscNumber: 1, TrackNumber: 2}}}},
		{name: "duplicate position", input: CreateDraftInput{CreatorID: uuid.New(), Title: "Title", Type: TypeEP, ReleaseDate: validDate, Tracks: []TrackInput{{Title: "One", DiscNumber: 1, TrackNumber: 1}, {Title: "Two", DiscNumber: 1, TrackNumber: 1}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := &authorizerStub{}
			_, err := NewService(&storeStub{}, authorizer).CreateDraft(
				context.Background(), uuid.New(), test.input,
			)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("CreateDraft() error = %v, want %v", err, ErrInvalid)
			}
			if authorizer.calls != 0 {
				t.Fatalf("Authorize() calls = %d", authorizer.calls)
			}
		})
	}
}

func TestServiceDoesNotCreateDraftWithoutPermission(t *testing.T) {
	store := &storeStub{}
	_, err := NewService(store, &authorizerStub{err: creator.ErrForbidden}).CreateDraft(
		context.Background(), uuid.New(), CreateDraftInput{
			CreatorID: uuid.New(), Title: "Denied", Type: TypeSingle,
			ReleaseDate: time.Now(),
			Tracks:      []TrackInput{{Title: "Denied", DiscNumber: 1, TrackNumber: 1}},
		},
	)
	if !errors.Is(err, creator.ErrForbidden) || store.createCalls != 0 {
		t.Fatalf("CreateDraft() error = %v, calls = %d", err, store.createCalls)
	}
}

func TestServiceReadsDraftForCreatorMember(t *testing.T) {
	actorID, creatorID, releaseID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{draft: Draft{Release: Release{
		ID: releaseID, CreatorID: creatorID, Status: StatusDraft,
	}}}
	authorizer := &authorizerStub{}
	draft, err := NewService(store, authorizer).ByID(
		context.Background(), actorID, releaseID,
	)
	if err != nil || draft.Release.ID != releaseID {
		t.Fatalf("ByID() = %+v, %v", draft, err)
	}
	if authorizer.actorID != actorID || authorizer.creatorID != creatorID ||
		authorizer.permission != creator.PermissionAccountRead {
		t.Fatalf("authorization = %+v", authorizer)
	}
}

func TestServiceRejectsInvalidReleaseIDBeforeReading(t *testing.T) {
	store := &storeStub{}
	_, err := NewService(store, &authorizerStub{}).ByID(
		context.Background(), uuid.New(), uuid.Nil,
	)
	if !errors.Is(err, ErrInvalid) || store.readCalls != 0 {
		t.Fatalf("ByID() error = %v, calls = %d", err, store.readCalls)
	}
}

type storeStub struct {
	draft       Draft
	createCalls int
	readCalls   int
}

func (store *storeStub) Create(_ context.Context, draft Draft) (Draft, error) {
	store.createCalls++
	store.draft = draft
	return draft, nil
}

func (store *storeStub) ByID(context.Context, uuid.UUID) (Draft, error) {
	store.readCalls++
	return store.draft, nil
}

type authorizerStub struct {
	actorID    uuid.UUID
	creatorID  uuid.UUID
	permission creator.Permission
	err        error
	calls      int
}

func (stub *authorizerStub) Authorize(
	_ context.Context,
	actorID uuid.UUID,
	creatorID uuid.UUID,
	permission creator.Permission,
) error {
	stub.calls++
	stub.actorID = actorID
	stub.creatorID = creatorID
	stub.permission = permission
	return stub.err
}

var _ Store = (*storeStub)(nil)
