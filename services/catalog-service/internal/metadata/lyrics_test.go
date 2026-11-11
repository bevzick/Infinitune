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
		{ID: uuid.New(), TrackID: trackID, Type: LyricsTypeSynchronized, Published: true, RightsAuthorized: true},
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

func TestReplaceTrackLyricsLinesUsesSynchronizedLyricsDuration(t *testing.T) {
	lyricsID := uuid.New()
	store := &lyricsStoreStub{
		timing: TrackLyrics{ID: lyricsID, Type: LyricsTypeSynchronized}, durationMS: 5000,
	}
	lines := []LyricsLine{
		{LineNumber: 1, StartMS: 0, EndMS: 2000, Text: " First line "},
		{LineNumber: 2, StartMS: 2000, EndMS: 5000, Text: "Second line"},
	}
	result, err := NewService(store).ReplaceTrackLyricsLines(
		context.Background(), lyricsID, lines,
	)
	if err != nil {
		t.Fatalf("ReplaceTrackLyricsLines() error = %v", err)
	}
	if store.replacedLyricsID != lyricsID || len(store.replaced) != 2 ||
		store.replaced[0].Text != "First line" || len(result) != 2 {
		t.Fatalf("result = %+v; stored = %+v", result, store.replaced)
	}
}

func TestReplaceTrackLyricsLinesRejectsPlainLyrics(t *testing.T) {
	store := &lyricsStoreStub{
		timing: TrackLyrics{ID: uuid.New(), Type: LyricsTypePlain}, durationMS: 5000,
	}
	_, err := NewService(store).ReplaceTrackLyricsLines(
		context.Background(), store.timing.ID,
		[]LyricsLine{{LineNumber: 1, StartMS: 0, EndMS: 1000, Text: "Line"}},
	)
	if !errors.Is(err, ErrInvalid) || store.replacedLyricsID != uuid.Nil {
		t.Fatalf("ReplaceTrackLyricsLines() error = %v; replaced id = %s",
			err, store.replacedLyricsID)
	}
}

type lyricsStoreStub struct {
	Store
	public           []TrackLyrics
	saved            TrackLyrics
	savedInput       TrackLyricsInput
	timing           TrackLyrics
	durationMS       int
	replacedLyricsID uuid.UUID
	replaced         []LyricsLine
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

func (stub *lyricsStoreStub) TrackLyricsTiming(
	context.Context,
	uuid.UUID,
) (TrackLyrics, int, error) {
	return stub.timing, stub.durationMS, nil
}

func (stub *lyricsStoreStub) ReplaceLyricsLines(
	_ context.Context,
	lyricsID uuid.UUID,
	lines []LyricsLine,
) ([]LyricsLine, error) {
	stub.replacedLyricsID = lyricsID
	stub.replaced = append([]LyricsLine(nil), lines...)
	return stub.replaced, nil
}
