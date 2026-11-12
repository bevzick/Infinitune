package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestArtistDiscographyValidatesArtistAndPagination(t *testing.T) {
	t.Parallel()

	store := &discographyStoreStub{}
	service := NewService(store)
	if _, err := service.ArtistDiscography(
		context.Background(), uuid.Nil, PageRequest{Limit: 20},
	); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ArtistDiscography(nil artist) error = %v, want %v", err, ErrInvalid)
	}
	if _, err := service.ArtistDiscography(
		context.Background(), uuid.New(), PageRequest{Limit: 0},
	); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ArtistDiscography(invalid page) error = %v, want %v", err, ErrInvalid)
	}
	if store.artistCalls != 0 || store.discographyCalls != 0 {
		t.Fatalf("store calls = artist %d, discography %d", store.artistCalls, store.discographyCalls)
	}
}

func TestArtistDiscographyRequiresExistingArtist(t *testing.T) {
	t.Parallel()

	store := &discographyStoreStub{artistErr: ErrNotFound}
	_, err := NewService(store).ArtistDiscography(
		context.Background(), uuid.New(), PageRequest{Limit: 20},
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ArtistDiscography() error = %v, want %v", err, ErrNotFound)
	}
	if store.artistCalls != 1 || store.discographyCalls != 0 {
		t.Fatalf("store calls = artist %d, discography %d", store.artistCalls, store.discographyCalls)
	}
}

func TestArtistDiscographyReturnsMixedReleasePage(t *testing.T) {
	t.Parallel()

	artistID := uuid.New()
	page := PageRequest{Limit: 10, Offset: 5}
	want := Page[DiscographyItem]{
		Items: []DiscographyItem{
			{Kind: DiscographyKindAlbum, Album: &Album{ID: uuid.New()}},
			{Kind: DiscographyKindTrack, Track: &Track{ID: uuid.New()}},
		},
		Limit: page.Limit, Offset: page.Offset, Total: 12,
	}
	store := &discographyStoreStub{page: want}
	got, err := NewService(store).ArtistDiscography(context.Background(), artistID, page)
	if err != nil {
		t.Fatalf("ArtistDiscography() error = %v", err)
	}
	if store.artistID != artistID || store.pageRequest != page ||
		len(got.Items) != 2 || got.Total != want.Total {
		t.Fatalf("ArtistDiscography() = %+v; store artist = %s, page = %+v", got, store.artistID, store.pageRequest)
	}
}

type discographyStoreStub struct {
	Store
	artistErr        error
	page             Page[DiscographyItem]
	artistID         uuid.UUID
	pageRequest      PageRequest
	artistCalls      int
	discographyCalls int
}

func (stub *discographyStoreStub) ArtistByID(
	_ context.Context,
	artistID uuid.UUID,
) (Artist, error) {
	stub.artistCalls++
	stub.artistID = artistID
	return Artist{ID: artistID}, stub.artistErr
}

func (stub *discographyStoreStub) ListArtistDiscography(
	_ context.Context,
	artistID uuid.UUID,
	page PageRequest,
) (Page[DiscographyItem], error) {
	stub.discographyCalls++
	stub.artistID = artistID
	stub.pageRequest = page
	return stub.page, nil
}
