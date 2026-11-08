package metadata

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const maximumPageSize = 100

const maximumTrackIDBatchSize = 100

func validatePage(page PageRequest) error {
	if page.Limit < 1 || page.Limit > maximumPageSize {
		return validationError("limit", "must be between 1 and 100")
	}
	if page.Offset < 0 {
		return validationError("offset", "must not be negative")
	}
	return nil
}

func validateTrackQuery(query TrackQuery) (TrackQuery, error) {
	if err := validatePage(query.Page); err != nil {
		return TrackQuery{}, err
	}
	query.Title = strings.TrimSpace(query.Title)
	if utf8.RuneCountInString(query.Title) > 200 {
		return TrackQuery{}, validationError("title", "must not exceed 200 characters")
	}
	for _, reference := range []struct {
		field string
		id    *uuid.UUID
	}{
		{field: "artist_id", id: query.ArtistID},
		{field: "album_id", id: query.AlbumID},
		{field: "genre_id", id: query.GenreID},
		{field: "mood_id", id: query.MoodID},
	} {
		if reference.id != nil && *reference.id == uuid.Nil {
			return TrackQuery{}, validationError(reference.field, "must not be empty")
		}
	}
	if query.ReleaseYear != nil && (*query.ReleaseYear < 1 || *query.ReleaseYear > 9999) {
		return TrackQuery{}, validationError("release_year", "must be between 1 and 9999")
	}
	if query.Sort == "" {
		query.Sort = TrackSortNewest
	}
	switch query.Sort {
	case TrackSortNewest, TrackSortOldest, TrackSortTitleAsc, TrackSortTitleDesc:
	default:
		return TrackQuery{}, validationError(
			"sort",
			"must be one of newest, oldest, title_asc, title_desc",
		)
	}
	return query, nil
}

func validateTrackIDs(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return validationError("track_ids", "must contain at least one id")
	}
	if len(ids) > maximumTrackIDBatchSize {
		return validationError("track_ids", "must not contain more than 100 ids")
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return validationError("track_ids", "contains an empty id")
		}
		if _, exists := seen[id]; exists {
			return validationError("track_ids", "contains a duplicate id")
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateArtist(input ArtistInput) (ArtistInput, error) {
	if err := validateID("id", input.ID); err != nil {
		return ArtistInput{}, err
	}
	name, err := requiredText("name", input.Name, 150)
	if err != nil {
		return ArtistInput{}, err
	}
	imageURL, err := optionalURL("image_url", input.ImageURL)
	if err != nil {
		return ArtistInput{}, err
	}
	input.Name = name
	input.Description = strings.TrimSpace(input.Description)
	input.ImageURL = imageURL
	return input, nil
}

func validateGenre(input GenreInput) (GenreInput, error) {
	if err := validateID("id", input.ID); err != nil {
		return GenreInput{}, err
	}
	name, err := requiredText("name", input.Name, 80)
	if err != nil {
		return GenreInput{}, err
	}
	slug, err := validateSlug(input.Slug)
	if err != nil {
		return GenreInput{}, err
	}
	input.Name = name
	input.Slug = slug
	return input, nil
}

func validateMood(input MoodInput) (MoodInput, error) {
	if err := validateID("id", input.ID); err != nil {
		return MoodInput{}, err
	}
	name, err := requiredText("name", input.Name, 80)
	if err != nil {
		return MoodInput{}, err
	}
	slug, err := validateSlug(input.Slug)
	if err != nil {
		return MoodInput{}, err
	}
	input.Name = name
	input.Slug = slug
	return input, nil
}

func validateAlbum(input AlbumInput) (AlbumInput, error) {
	if err := validateID("id", input.ID); err != nil {
		return AlbumInput{}, err
	}
	title, err := requiredText("title", input.Title, 200)
	if err != nil {
		return AlbumInput{}, err
	}
	if !input.Type.valid() {
		return AlbumInput{}, validationError("album_type", "unsupported value")
	}
	if input.ReleaseDate.IsZero() {
		return AlbumInput{}, validationError("release_date", "is required")
	}
	coverURL, err := optionalURL("cover_url", input.CoverURL)
	if err != nil {
		return AlbumInput{}, err
	}
	if err = validateArtistPositions("artists", input.Artists); err != nil {
		return AlbumInput{}, err
	}

	input.Title = title
	input.ReleaseDate = dateOnly(input.ReleaseDate)
	input.CoverURL = coverURL
	return input, nil
}

func validateTrack(input TrackInput) (TrackInput, error) {
	if err := validateID("id", input.ID); err != nil {
		return TrackInput{}, err
	}
	if input.AlbumID != nil {
		if err := validateID("album_id", *input.AlbumID); err != nil {
			return TrackInput{}, err
		}
	}
	title, err := requiredText("title", input.Title, 200)
	if err != nil {
		return TrackInput{}, err
	}
	if input.DurationMS <= 0 || int64(input.DurationMS) > math.MaxInt32 {
		return TrackInput{}, validationError("duration_ms", "must be between 1 and 2147483647")
	}
	if input.TrackNumber <= 0 {
		return TrackInput{}, validationError("track_number", "must be positive")
	}
	if input.DiscNumber <= 0 {
		return TrackInput{}, validationError("disc_number", "must be positive")
	}
	if input.ReleaseDate.IsZero() {
		return TrackInput{}, validationError("release_date", "is required")
	}
	if err = validateID("media_asset_id", input.MediaAssetID); err != nil {
		return TrackInput{}, err
	}
	if err = validateID("content_id", input.ContentID); err != nil {
		return TrackInput{}, err
	}
	if err = validateTrackArtists(input.Artists); err != nil {
		return TrackInput{}, err
	}
	if err = validateGenreIDs(input.GenreIDs); err != nil {
		return TrackInput{}, err
	}
	if err = validateMoods(input.Moods); err != nil {
		return TrackInput{}, err
	}

	input.Title = title
	input.ReleaseDate = dateOnly(input.ReleaseDate)
	return input, nil
}

func validateArtistPositions(field string, artists []ArtistPositionInput) error {
	if len(artists) == 0 {
		return validationError(field, "must contain at least one artist")
	}
	artistIDs := make(map[uuid.UUID]struct{}, len(artists))
	positions := make(map[int16]struct{}, len(artists))
	for _, artist := range artists {
		if artist.ArtistID == uuid.Nil {
			return validationError(field, "contains an empty artist id")
		}
		if artist.Position <= 0 {
			return validationError(field, "positions must be positive")
		}
		if _, exists := artistIDs[artist.ArtistID]; exists {
			return validationError(field, "contains a duplicate artist")
		}
		if _, exists := positions[artist.Position]; exists {
			return validationError(field, "contains a duplicate position")
		}
		artistIDs[artist.ArtistID] = struct{}{}
		positions[artist.Position] = struct{}{}
	}
	return nil
}

func validateTrackArtists(artists []TrackArtistInput) error {
	if len(artists) == 0 {
		return validationError("artists", "must contain at least one artist")
	}
	credits := make(map[string]struct{}, len(artists))
	positions := make(map[string]struct{}, len(artists))
	hasPrimary := false
	for _, artist := range artists {
		if artist.ArtistID == uuid.Nil {
			return validationError("artists", "contains an empty artist id")
		}
		if !artist.Role.valid() {
			return validationError("artists", "contains an unsupported role")
		}
		if artist.Position <= 0 {
			return validationError("artists", "positions must be positive")
		}
		if artist.Role == ArtistRolePrimary {
			hasPrimary = true
		}
		creditKey := artist.ArtistID.String() + ":" + string(artist.Role)
		positionKey := string(artist.Role) + ":" + fmt.Sprint(artist.Position)
		if _, exists := credits[creditKey]; exists {
			return validationError("artists", "contains a duplicate artist role")
		}
		if _, exists := positions[positionKey]; exists {
			return validationError("artists", "contains a duplicate role position")
		}
		credits[creditKey] = struct{}{}
		positions[positionKey] = struct{}{}
	}
	if !hasPrimary {
		return validationError("artists", "must contain a primary artist")
	}
	return nil
}

func validateGenreIDs(ids []uuid.UUID) error {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return validationError("genre_ids", "contains an empty id")
		}
		if _, exists := seen[id]; exists {
			return validationError("genre_ids", "contains a duplicate id")
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateMoods(moods []TrackMoodInput) error {
	seen := make(map[uuid.UUID]struct{}, len(moods))
	for _, mood := range moods {
		if mood.MoodID == uuid.Nil {
			return validationError("moods", "contains an empty mood id")
		}
		if math.IsNaN(mood.Weight) || math.IsInf(mood.Weight, 0) ||
			mood.Weight < 0 || mood.Weight > 1 {
			return validationError("moods", "weights must be between 0 and 1")
		}
		if _, exists := seen[mood.MoodID]; exists {
			return validationError("moods", "contains a duplicate mood")
		}
		seen[mood.MoodID] = struct{}{}
	}
	return nil
}

func validateID(field string, id uuid.UUID) error {
	if id == uuid.Nil {
		return validationError(field, "must not be empty")
	}
	return nil
}

func requiredText(field, value string, maxRunes int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", validationError(field, "must not be blank")
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return "", validationError(field, fmt.Sprintf("must not exceed %d characters", maxRunes))
	}
	return value, nil
}

func optionalURL(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.User != nil {
		return "", validationError(field, "must be an absolute HTTP(S) URL")
	}
	return parsed.String(), nil
}

func validateSlug(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", validationError("slug", "must not be blank")
	}
	if utf8.RuneCountInString(value) > 80 || !slugPattern.MatchString(value) {
		return "", validationError("slug", "must contain lowercase words separated by hyphens")
	}
	return value, nil
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

func (value AlbumType) valid() bool {
	switch value {
	case AlbumTypeAlbum, AlbumTypeSingle, AlbumTypeEP, AlbumTypeCompilation:
		return true
	default:
		return false
	}
}

func (value ArtistRole) valid() bool {
	switch value {
	case ArtistRolePrimary, ArtistRoleFeatured, ArtistRoleProducer, ArtistRoleComposer:
		return true
	default:
		return false
	}
}
