package metadata

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateTrackCanonicalizesApprovedMetadata(t *testing.T) {
	primaryArtistID := uuid.New()
	releaseDate := time.Date(2026, time.November, 8, 18, 30, 0, 0, time.FixedZone("test", 2*60*60))
	input := TrackInput{
		ID:           uuid.New(),
		Title:        "  Approved Track  ",
		DurationMS:   180000,
		TrackNumber:  1,
		DiscNumber:   1,
		ReleaseDate:  releaseDate,
		MediaAssetID: uuid.New(),
		ContentID:    uuid.New(),
		Artists: []TrackArtistInput{
			{ArtistID: primaryArtistID, Role: ArtistRolePrimary, Position: 1},
			{ArtistID: primaryArtistID, Role: ArtistRoleComposer, Position: 1},
		},
		GenreIDs: []uuid.UUID{uuid.New(), uuid.New()},
		Moods:    []TrackMoodInput{{MoodID: uuid.New(), Weight: 0.75}},
	}

	canonical, err := validateTrack(input)
	if err != nil {
		t.Fatalf("validateTrack() error = %v", err)
	}
	if canonical.Title != "Approved Track" {
		t.Errorf("Title = %q, want Approved Track", canonical.Title)
	}
	wantDate := time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC)
	if !canonical.ReleaseDate.Equal(wantDate) {
		t.Errorf("ReleaseDate = %v, want %v", canonical.ReleaseDate, wantDate)
	}
}

func TestValidateTrackRejectsUnsafeMetadata(t *testing.T) {
	valid := validTrackInput()
	tests := []struct {
		name   string
		mutate func(*TrackInput)
		field  string
	}{
		{
			name: "missing primary artist",
			mutate: func(input *TrackInput) {
				input.Artists[0].Role = ArtistRoleFeatured
			},
			field: "artists",
		},
		{
			name: "duplicate genre",
			mutate: func(input *TrackInput) {
				input.GenreIDs = append(input.GenreIDs, input.GenreIDs[0])
			},
			field: "genre_ids",
		},
		{
			name: "invalid mood weight",
			mutate: func(input *TrackInput) {
				input.Moods[0].Weight = math.NaN()
			},
			field: "moods",
		},
		{
			name: "duplicate role position",
			mutate: func(input *TrackInput) {
				input.Artists = append(input.Artists, TrackArtistInput{
					ArtistID: uuid.New(), Role: ArtistRolePrimary, Position: 1,
				})
			},
			field: "artists",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			input.Artists = append([]TrackArtistInput(nil), valid.Artists...)
			input.GenreIDs = append([]uuid.UUID(nil), valid.GenreIDs...)
			input.Moods = append([]TrackMoodInput(nil), valid.Moods...)
			test.mutate(&input)

			_, err := validateTrack(input)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateTrack() error = %v, want %v", err, ErrInvalid)
			}
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("validation error = %#v, want field %q", validationError, test.field)
			}
		})
	}
}

func TestValidateTrackLyricsRequiresSeparatePublicationRights(t *testing.T) {
	input := TrackLyricsInput{
		ID: uuid.New(), TrackID: uuid.New(), Type: LyricsTypePlain,
		Language: " UK ", Content: "  Текст пісні  ",
		SourceType: LyricsSourcePublisher, SourceName: " Publisher ",
		CopyrightNotice: " Licensed ", Official: true,
		RightsAuthorized: true, Published: true,
	}
	canonical, err := validateTrackLyrics(input)
	if err != nil {
		t.Fatalf("validateTrackLyrics() error = %v", err)
	}
	if canonical.Language != "uk" || canonical.Content != "Текст пісні" ||
		canonical.SourceName != "Publisher" || canonical.CopyrightNotice != "Licensed" {
		t.Fatalf("validateTrackLyrics() = %+v", canonical)
	}

	input.RightsAuthorized = false
	_, err = validateTrackLyrics(input)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("validateTrackLyrics() error = %v, want %v", err, ErrInvalid)
	}
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "published" {
		t.Fatalf("validation error = %#v, want published field", validationError)
	}
}

func TestValidateLyricsLinesRejectsInvalidPlaybackRanges(t *testing.T) {
	valid := []LyricsLine{
		{LineNumber: 1, StartMS: 0, EndMS: 1000, Text: "First"},
		{LineNumber: 2, StartMS: 1000, EndMS: 2000, Text: "Second"},
	}
	if _, err := validateLyricsLines(valid, 2000); err != nil {
		t.Fatalf("validateLyricsLines() error = %v", err)
	}
	tests := []struct {
		name  string
		lines []LyricsLine
	}{
		{name: "missing line number", lines: []LyricsLine{{LineNumber: 2, StartMS: 0, EndMS: 1000, Text: "Line"}}},
		{name: "overlap", lines: []LyricsLine{
			{LineNumber: 1, StartMS: 0, EndMS: 1000, Text: "First"},
			{LineNumber: 2, StartMS: 900, EndMS: 1500, Text: "Second"},
		}},
		{name: "past duration", lines: []LyricsLine{{LineNumber: 1, StartMS: 0, EndMS: 2001, Text: "Line"}}},
		{name: "blank text", lines: []LyricsLine{{LineNumber: 1, StartMS: 0, EndMS: 1000, Text: " "}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := validateLyricsLines(test.lines, 2000); !errors.Is(err, ErrInvalid) {
				t.Fatalf("validateLyricsLines() error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}

func TestValidateAlbumSupportsSingleWithMultipleArtists(t *testing.T) {
	input := AlbumInput{
		ID:          uuid.New(),
		Title:       "  Shared Single ",
		Type:        AlbumTypeSingle,
		ReleaseDate: time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
		CoverURL:    " https://cdn.example.test/cover.jpg ",
		Artists: []ArtistPositionInput{
			{ArtistID: uuid.New(), Position: 1},
			{ArtistID: uuid.New(), Position: 2},
		},
	}

	canonical, err := validateAlbum(input)
	if err != nil {
		t.Fatalf("validateAlbum() error = %v", err)
	}
	if canonical.Type != AlbumTypeSingle || canonical.Title != "Shared Single" || len(canonical.Artists) != 2 {
		t.Fatalf("validateAlbum() = %+v", canonical)
	}
}

func TestValidateTaxonomyCanonicalizesSlug(t *testing.T) {
	genre, err := validateGenre(GenreInput{ID: uuid.New(), Name: " Electronic ", Slug: " Electronic-Music "})
	if err != nil {
		t.Fatalf("validateGenre() error = %v", err)
	}
	if genre.Name != "Electronic" || genre.Slug != "electronic-music" {
		t.Fatalf("validateGenre() = %+v", genre)
	}
}

func TestValidatePage(t *testing.T) {
	t.Parallel()

	if err := validatePage(PageRequest{Limit: 100, Offset: 10}); err != nil {
		t.Fatalf("validatePage() error = %v", err)
	}
	for _, page := range []PageRequest{
		{Limit: 0},
		{Limit: 101},
		{Limit: 20, Offset: -1},
	} {
		if err := validatePage(page); !errors.Is(err, ErrInvalid) {
			t.Fatalf("validatePage(%+v) error = %v, want %v", page, err, ErrInvalid)
		}
	}
}

func TestValidateTrackQuery(t *testing.T) {
	t.Parallel()

	year := 2026
	query, err := validateTrackQuery(TrackQuery{
		Page: PageRequest{Limit: 20}, Title: "  Track Name  ", ReleaseYear: &year,
	})
	if err != nil {
		t.Fatalf("validateTrackQuery() error = %v", err)
	}
	if query.Title != "Track Name" || query.Sort != TrackSortNewest {
		t.Fatalf("validateTrackQuery() = %+v", query)
	}

	invalidYear := 10000
	for _, invalid := range []TrackQuery{
		{Page: PageRequest{Limit: 20}, Sort: "random"},
		{Page: PageRequest{Limit: 20}, ReleaseYear: &invalidYear},
		{Page: PageRequest{Limit: 20}, ArtistID: new(uuid.UUID)},
	} {
		if _, err = validateTrackQuery(invalid); !errors.Is(err, ErrInvalid) {
			t.Fatalf("validateTrackQuery(%+v) error = %v, want %v", invalid, err, ErrInvalid)
		}
	}
}

func TestValidateTrackIDs(t *testing.T) {
	t.Parallel()

	firstID := uuid.New()
	if err := validateTrackIDs([]uuid.UUID{firstID, uuid.New()}); err != nil {
		t.Fatalf("validateTrackIDs() error = %v", err)
	}
	tooMany := make([]uuid.UUID, 101)
	for index := range tooMany {
		tooMany[index] = uuid.New()
	}
	for _, ids := range [][]uuid.UUID{
		nil,
		{uuid.Nil},
		{firstID, firstID},
		tooMany,
	} {
		if err := validateTrackIDs(ids); !errors.Is(err, ErrInvalid) {
			t.Fatalf("validateTrackIDs(%d ids) error = %v, want %v", len(ids), err, ErrInvalid)
		}
	}
}

func validTrackInput() TrackInput {
	return TrackInput{
		ID:           uuid.New(),
		Title:        "Valid Track",
		DurationMS:   180000,
		TrackNumber:  1,
		DiscNumber:   1,
		ReleaseDate:  time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
		MediaAssetID: uuid.New(),
		ContentID:    uuid.New(),
		Artists: []TrackArtistInput{
			{ArtistID: uuid.New(), Role: ArtistRolePrimary, Position: 1},
		},
		GenreIDs: []uuid.UUID{uuid.New()},
		Moods:    []TrackMoodInput{{MoodID: uuid.New(), Weight: 0.5}},
	}
}
