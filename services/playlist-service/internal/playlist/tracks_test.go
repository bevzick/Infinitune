package playlist

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestTrackServiceAddsValidatedTrackForOwner(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	trackID := uuid.New()
	playlists := &stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: ownerID, Type: TypeManual,
	}}
	tracks := &trackStoreStub{}
	validator := &trackValidatorStub{}
	service := NewTrackService(NewService(playlists), tracks, validator)

	added, err := service.Add(context.Background(), ownerID, playlistID, trackID)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if validator.trackID != trackID {
		t.Fatalf("ValidateTrack() id = %s, want %s", validator.trackID, trackID)
	}
	if added.PlaylistID != playlistID || added.TrackID != trackID || added.AddedBy != ownerID {
		t.Fatalf("Add() = %+v", added)
	}
}

func TestTrackServiceDoesNotAddMissingCatalogTrack(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	tracks := &trackStoreStub{}
	service := NewTrackService(
		NewService(&stubStore{playlist: Playlist{
			ID: playlistID, OwnerID: ownerID, Type: TypeManual,
		}}),
		tracks,
		&trackValidatorStub{err: ErrTrackNotFound},
	)

	_, err := service.Add(context.Background(), ownerID, playlistID, uuid.New())
	if !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("Add() error = %v, want %v", err, ErrTrackNotFound)
	}
	if tracks.added {
		t.Fatal("Add() persisted a track rejected by Catalog")
	}
}

func TestTrackServiceEnforcesOwnerAndManualType(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	trackID := uuid.New()
	tests := []struct {
		name     string
		actorID  uuid.UUID
		typeName Type
		want     error
	}{
		{name: "different owner", actorID: uuid.New(), typeName: TypeManual, want: ErrForbidden},
		{name: "dynamic playlist", actorID: ownerID, typeName: TypeDynamic, want: ErrManualOnly},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewTrackService(NewService(&stubStore{playlist: Playlist{
				ID: playlistID, OwnerID: ownerID, Type: test.typeName,
			}}), &trackStoreStub{}, &trackValidatorStub{})
			_, err := service.Add(context.Background(), test.actorID, playlistID, trackID)
			if !errors.Is(err, test.want) {
				t.Fatalf("Add() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestTrackServiceReadsTracksUsingPlaylistVisibility(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	trackID := uuid.New()
	tracks := &trackStoreStub{tracks: TrackPage{
		Items: []Track{{PlaylistID: playlistID, TrackID: trackID, Position: 1}}, Total: 1,
	}}

	privateService := NewTrackService(NewService(&stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: ownerID, Type: TypeManual, Visibility: VisibilityPrivate,
	}}), tracks, &trackValidatorStub{})
	if _, err := privateService.Tracks(context.Background(), uuid.New(), playlistID, PageRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Tracks(private) error = %v, want %v", err, ErrNotFound)
	}

	publicService := NewTrackService(NewService(&stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: ownerID, Type: TypeManual, Visibility: VisibilityPublic,
	}}), tracks, &trackValidatorStub{})
	result, err := publicService.Tracks(context.Background(), uuid.Nil, playlistID, PageRequest{})
	if err != nil {
		t.Fatalf("Tracks(public) error = %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].TrackID != trackID || result.Limit != defaultTrackPageLimit {
		t.Fatalf("Tracks(public) = %+v", result)
	}
}

func TestTrackServiceDoesNotReadStoredTracksForDynamicPlaylist(t *testing.T) {
	playlistID := uuid.New()
	service := NewTrackService(NewService(&stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: uuid.New(), Type: TypeDynamic, Visibility: VisibilityPublic,
	}}), &trackStoreStub{}, &trackValidatorStub{})

	_, err := service.Tracks(context.Background(), uuid.Nil, playlistID, PageRequest{})
	if !errors.Is(err, ErrManualOnly) {
		t.Fatalf("Tracks(dynamic) error = %v, want %v", err, ErrManualOnly)
	}
}

func TestTrackServiceValidatesCompleteOrder(t *testing.T) {
	ownerID := uuid.New()
	playlistID := uuid.New()
	trackID := uuid.New()
	service := NewTrackService(NewService(&stubStore{playlist: Playlist{
		ID: playlistID, OwnerID: ownerID, Type: TypeManual,
	}}), &trackStoreStub{}, &trackValidatorStub{})

	for _, order := range [][]uuid.UUID{{uuid.Nil}, {trackID, trackID}} {
		if _, err := service.Reorder(context.Background(), ownerID, playlistID, order); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("Reorder(%v) error = %v, want %v", order, err, ErrInvalidArgument)
		}
	}
}

type trackValidatorStub struct {
	trackID uuid.UUID
	err     error
}

func (stub *trackValidatorStub) ValidateTrack(_ context.Context, trackID uuid.UUID) error {
	stub.trackID = trackID
	return stub.err
}

type trackStoreStub struct {
	tracks TrackPage
	added  bool
}

func (stub *trackStoreStub) Tracks(
	_ context.Context,
	_ uuid.UUID,
	page PageRequest,
) (TrackPage, error) {
	stub.tracks.Limit = page.Limit
	stub.tracks.Offset = page.Offset
	return stub.tracks, nil
}

func (stub *trackStoreStub) AddTrack(
	_ context.Context,
	playlistID uuid.UUID,
	_ uuid.UUID,
	trackID uuid.UUID,
	addedBy uuid.UUID,
) (Track, error) {
	stub.added = true
	return Track{PlaylistID: playlistID, TrackID: trackID, Position: 1, AddedBy: addedBy}, nil
}

func (stub *trackStoreStub) RemoveTrack(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

func (stub *trackStoreStub) ReorderTracks(
	_ context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
	trackIDs []uuid.UUID,
) ([]Track, error) {
	result := make([]Track, 0, len(trackIDs))
	for index, trackID := range trackIDs {
		result = append(result, Track{TrackID: trackID, Position: index + 1})
	}
	return result, nil
}
