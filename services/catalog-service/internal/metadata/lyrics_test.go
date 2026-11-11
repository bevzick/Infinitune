package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestTrackLyricsReturnsOnlyPublishedAuthorizedValues(t *testing.T) {
	trackID := uuid.New()
	store := &lyricsStoreStub{public: []TrackLyrics{
		{ID: uuid.New(), TrackID: trackID, Published: true, RightsAuthorized: true},
		{ID: uuid.New(), TrackID: trackID, Published: false, RightsAuthorized: true},
		{ID: uuid.New(), TrackID: trackID, Published: false, RightsAuthorized: false},
	}}
	result, err := NewService(store).TrackLyrics(context.Background(), trackID)
	if err != nil {
		t.Fatalf("TrackLyrics() error = %v", err)
	}
	if len(result) != 1 || !result[0].Published || !result[0].RightsAuthorized {
		t.Fatalf("TrackLyrics() = %+v", result)
	}
}

func TestPutTrackLyricsRejectsPublicationWithoutLyricsRights(t *testing.T) {
	store := &lyricsStoreStub{}
	_, err := NewService(store).PutTrackLyrics(context.Background(), TrackLyricsInput{
		ID: uuid.New(), TrackID: uuid.New(), Type: LyricsTypePlain,
		Language: "en", Content: "Lyrics", SourceType: LyricsSourceArtist,
		SourceName: "Artist", Published: true,
	})
	if !errors.Is(err, ErrInvalid) || store.savedInput.ID != uuid.Nil {
		t.Fatalf("PutTrackLyrics() error = %v; saved input = %+v", err, store.savedInput)
	}
}

type lyricsStoreStub struct {
	Store
	public     []TrackLyrics
	saved      TrackLyrics
	savedInput TrackLyricsInput
}

func (stub *lyricsStoreStub) PublicTrackLyrics(context.Context, uuid.UUID) ([]TrackLyrics, error) {
	return stub.public, nil
}

func (stub *lyricsStoreStub) SaveTrackLyrics(
	_ context.Context,
	input TrackLyricsInput,
) (TrackLyrics, error) {
	stub.savedInput = input
	return stub.saved, nil
}
