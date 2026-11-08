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
