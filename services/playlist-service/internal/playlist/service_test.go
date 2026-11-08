package playlist

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCreateDerivesOwnerFromActor(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)
	actorID := uuid.New()

	created, err := service.Create(context.Background(), actorID, CreateInput{
		Name:        "  My playlist  ",
		Description: "description",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("Create().ID is nil")
	}
	if created.OwnerID != actorID {
		t.Fatalf("Create().OwnerID = %s, want actor %s", created.OwnerID, actorID)
	}
	if created.Name != "My playlist" || created.Type != TypeManual ||
		created.Visibility != VisibilityPrivate {
		t.Fatalf("Create() = %+v", created)
	}
}

func TestByIDAppliesVisibilityPolicy(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	playlistID := uuid.New()
	tests := []struct {
		name       string
		visibility Visibility
		viewerID   uuid.UUID
		wantError  error
	}{
		{name: "owner reads private", visibility: VisibilityPrivate, viewerID: ownerID},
		{name: "private is hidden from another user", visibility: VisibilityPrivate, viewerID: otherID, wantError: ErrNotFound},
		{name: "private is hidden from anonymous user", visibility: VisibilityPrivate, wantError: ErrNotFound},
		{name: "unlisted is readable by id", visibility: VisibilityUnlisted},
		{name: "public is readable by id", visibility: VisibilityPublic, viewerID: otherID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(&stubStore{playlist: Playlist{
				ID: playlistID, OwnerID: ownerID, Visibility: test.visibility,
			}})
			_, err := service.ByID(context.Background(), test.viewerID, playlistID)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("ByID() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestUpdateAllowsOnlyOwnerAndKeepsOwner(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	store := &stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: ownerID, Name: "Old", Type: TypeManual,
		Visibility: VisibilityPrivate,
	}}
	service := NewService(store)
	name := "  New name "
	visibility := VisibilityPublic

	updated, err := service.Update(context.Background(), ownerID, playlistID, UpdateInput{
		Name: &name, Visibility: &visibility,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.OwnerID != ownerID || updated.Name != "New name" ||
		updated.Visibility != VisibilityPublic {
		t.Fatalf("Update() = %+v", updated)
	}
	_, err = service.Update(context.Background(), uuid.New(), playlistID, UpdateInput{Name: &name})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update() by non-owner error = %v, want %v", err, ErrForbidden)
	}
}

func TestDeleteAllowsOnlyOwner(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	store := &stubStore{playlist: Playlist{ID: playlistID, OwnerID: ownerID}}
	service := NewService(store)

	if err := service.Delete(context.Background(), uuid.New(), playlistID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete() by non-owner error = %v, want %v", err, ErrForbidden)
	}
	if store.deleted {
		t.Fatal("Delete() called store for a non-owner")
	}
	if err := service.Delete(context.Background(), ownerID, playlistID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !store.deleted {
		t.Fatal("Delete() did not call store for the owner")
	}
}

func TestCreateValidatesInput(t *testing.T) {
	tests := []struct {
		name    string
		actorID uuid.UUID
		input   CreateInput
	}{
		{name: "missing actor", input: CreateInput{Name: "valid"}},
		{name: "blank name", actorID: uuid.New(), input: CreateInput{Name: "  "}},
		{name: "long name", actorID: uuid.New(), input: CreateInput{Name: strings.Repeat("я", 151)}},
		{name: "invalid visibility", actorID: uuid.New(), input: CreateInput{Name: "valid", Visibility: "friends"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(&stubStore{}).Create(context.Background(), test.actorID, test.input)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("Create() error = %v, want %v", err, ErrInvalidArgument)
			}
		})
	}
}

type stubStore struct {
	playlist Playlist
	deleted  bool
}

func (store *stubStore) Create(_ context.Context, value Playlist) (Playlist, error) {
	store.playlist = value
	return value, nil
}

func (store *stubStore) ByID(_ context.Context, _ uuid.UUID) (Playlist, error) {
	return store.playlist, nil
}

func (store *stubStore) Update(_ context.Context, value Playlist) (Playlist, error) {
	store.playlist = value
	return value, nil
}

func (store *stubStore) Delete(_ context.Context, _, _ uuid.UUID) error {
	store.deleted = true
	return nil
}
