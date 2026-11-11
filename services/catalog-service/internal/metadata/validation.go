package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	slugPattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	languageTagPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,8}(?:-[A-Za-z0-9]{1,8})*$`)
)

const maximumPageSize = 100

const maximumTrackIDBatchSize = 100

const maximumPublicationTracks = 500

func validatePublication(input PublicationInput) (PublicationInput, string, error) {
	if err := validateID("source_release_id", input.SourceReleaseID); err != nil {
		return PublicationInput{}, "", err
	}
	artist, err := validateArtist(input.Artist)
	if err != nil {
		return PublicationInput{}, "", err
	}
	input.Artist = artist
	if len(input.Tracks) == 0 || len(input.Tracks) > maximumPublicationTracks {
		return PublicationInput{}, "", validationError("tracks", "must contain from 1 to 500 tracks")
	}
	var albumID *uuid.UUID
	if input.Album != nil {
		album, albumErr := validateAlbum(*input.Album)
		if albumErr != nil {
			return PublicationInput{}, "", albumErr
		}
		if len(album.Artists) != 1 || album.Artists[0].ArtistID != artist.ID ||
			album.Artists[0].Position != 1 {
			return PublicationInput{}, "", validationError("album.artists", "must contain the publishing artist at position 1")
		}
		input.Album = &album
		albumID = &album.ID
	}
	trackIDs := make(map[uuid.UUID]struct{}, len(input.Tracks))
	contentIDs := make(map[uuid.UUID]struct{}, len(input.Tracks))
	mediaIDs := make(map[uuid.UUID]struct{}, len(input.Tracks))
	tracks := make([]TrackInput, 0, len(input.Tracks))
	for index, track := range input.Tracks {
		canonical, trackErr := validateTrack(track)
		if trackErr != nil {
			return PublicationInput{}, "", fmt.Errorf("tracks[%d]: %w", index, trackErr)
		}
		if !sameOptionalUUID(canonical.AlbumID, albumID) {
			return PublicationInput{}, "", validationError(
				fmt.Sprintf("tracks[%d].album_id", index), "must match the published album",
			)
		}
		if len(canonical.Artists) != 1 || canonical.Artists[0].ArtistID != artist.ID ||
			canonical.Artists[0].Role != ArtistRolePrimary || canonical.Artists[0].Position != 1 {
			return PublicationInput{}, "", validationError(
				fmt.Sprintf("tracks[%d].artists", index), "must contain the publishing artist as primary",
			)
		}
		for _, unique := range []struct {
			field string
			id    uuid.UUID
			seen  map[uuid.UUID]struct{}
		}{
			{field: "id", id: canonical.ID, seen: trackIDs},
			{field: "content_id", id: canonical.ContentID, seen: contentIDs},
			{field: "media_asset_id", id: canonical.MediaAssetID, seen: mediaIDs},
		} {
			if _, exists := unique.seen[unique.id]; exists {
				return PublicationInput{}, "", validationError(
					fmt.Sprintf("tracks[%d].%s", index, unique.field), "must not be duplicated",
				)
			}
			unique.seen[unique.id] = struct{}{}
		}
		tracks = append(tracks, canonical)
	}
	input.Tracks = tracks
	encoded, err := json.Marshal(input)
	if err != nil {
		return PublicationInput{}, "", fmt.Errorf("encode publication fingerprint: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return input, hex.EncodeToString(digest[:]), nil
}

func sameOptionalUUID(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

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

func validateDynamicTrackQuery(query DynamicTrackQuery) (DynamicTrackQuery, error) {
	if err := validatePage(query.Page); err != nil {
		return DynamicTrackQuery{}, err
	}
	if query.Logic != FilterLogicAnd && query.Logic != FilterLogicOr {
		return DynamicTrackQuery{}, validationError("logic", "must be AND or OR")
	}
	if len(query.Filters) == 0 {
		return DynamicTrackQuery{}, validationError("filters", "must not be empty")
	}
	if query.Sort == "" {
		query.Sort = TrackSortNewest
	}
	switch query.Sort {
	case TrackSortNewest, TrackSortOldest, TrackSortTitleAsc, TrackSortTitleDesc:
	default:
		return DynamicTrackQuery{}, validationError(
			"sort", "must be one of newest, oldest, title_asc, title_desc",
		)
	}

	filters := make([]TrackFilter, 0, len(query.Filters))
	for index, filter := range query.Filters {
		canonical, err := validateTrackFilter(filter, index)
		if err != nil {
			return DynamicTrackQuery{}, err
		}
		filters = append(filters, canonical)
	}
	query.Filters = filters
	return query, nil
}

func validateTrackFilter(filter TrackFilter, index int) (TrackFilter, error) {
	path := fmt.Sprintf("filters[%d]", index)
	switch filter.Field {
	case TrackFilterTitle:
		if filter.Operator != TrackFilterContains && filter.Operator != TrackFilterEquals {
			return TrackFilter{}, validationError(path+".operator", "title supports contains or equals")
		}
		if filter.Value.Kind != TrackFilterValueText {
			return TrackFilter{}, filterValueKindError(path, TrackFilterValueText)
		}
		filter.Value.Text = strings.TrimSpace(filter.Value.Text)
		if filter.Value.Text == "" {
			return TrackFilter{}, validationError(path+".value", "must not be blank")
		}
		if !utf8.ValidString(filter.Value.Text) || utf8.RuneCountInString(filter.Value.Text) > 200 {
			return TrackFilter{}, validationError(path+".value", "must contain at most 200 characters")
		}
	case TrackFilterArtistID, TrackFilterAlbumID, TrackFilterGenreID, TrackFilterMoodID:
		if err := validateUUIDTrackFilter(filter, path); err != nil {
			return TrackFilter{}, err
		}
	case TrackFilterReleaseYear:
		if err := validateIntegerTrackFilter(filter, path, 1, 9999); err != nil {
			return TrackFilter{}, err
		}
	case TrackFilterDurationMS:
		if err := validateIntegerTrackFilter(filter, path, 1, math.MaxInt32); err != nil {
			return TrackFilter{}, err
		}
	case TrackFilterExplicit:
		if filter.Operator != TrackFilterEquals {
			return TrackFilter{}, validationError(path+".operator", "explicit supports equals")
		}
		if filter.Value.Kind != TrackFilterValueBoolean {
			return TrackFilter{}, filterValueKindError(path, TrackFilterValueBoolean)
		}
	default:
		return TrackFilter{}, validationError(path+".field", "is not supported")
	}
	filter.Value = canonicalTrackFilterValue(filter.Value)
	return filter, nil
}

func validateUUIDTrackFilter(filter TrackFilter, path string) error {
	switch filter.Operator {
	case TrackFilterEquals:
		if filter.Value.Kind != TrackFilterValueUUID {
			return filterValueKindError(path, TrackFilterValueUUID)
		}
		if filter.Value.UUID == uuid.Nil {
			return validationError(path+".value", "must be a UUID")
		}
	case TrackFilterIn:
		if filter.Value.Kind != TrackFilterValueUUIDList {
			return filterValueKindError(path, TrackFilterValueUUIDList)
		}
		if len(filter.Value.UUIDs) == 0 || len(filter.Value.UUIDs) > 100 {
			return validationError(path+".value", "must contain from 1 to 100 UUIDs")
		}
		seen := make(map[uuid.UUID]struct{}, len(filter.Value.UUIDs))
		for _, id := range filter.Value.UUIDs {
			if id == uuid.Nil {
				return validationError(path+".value", "must contain UUIDs")
			}
			if _, exists := seen[id]; exists {
				return validationError(path+".value", "must not contain duplicates")
			}
			seen[id] = struct{}{}
		}
	default:
		return validationError(path+".operator", "UUID fields support equals or in")
	}
	return nil
}

func validateIntegerTrackFilter(filter TrackFilter, path string, minimum, maximum int) error {
	switch filter.Operator {
	case TrackFilterEquals, TrackFilterGreaterThan, TrackFilterGreaterThanOrEqual,
		TrackFilterLessThan, TrackFilterLessThanOrEqual:
		if filter.Value.Kind != TrackFilterValueInteger {
			return filterValueKindError(path, TrackFilterValueInteger)
		}
		if filter.Value.Integer < minimum || filter.Value.Integer > maximum {
			return validationError(path+".value",
				fmt.Sprintf("must be from %d to %d", minimum, maximum))
		}
	case TrackFilterBetween:
		if filter.Value.Kind != TrackFilterValueIntegerRange {
			return filterValueKindError(path, TrackFilterValueIntegerRange)
		}
		value := filter.Value.IntegerRange
		if value.Minimum < minimum || value.Maximum > maximum || value.Minimum > value.Maximum {
			return validationError(path+".value",
				fmt.Sprintf("range must be ordered and within %d to %d", minimum, maximum))
		}
	default:
		return validationError(path+".operator", "numeric fields support equals, comparisons, or between")
	}
	return nil
}

func canonicalTrackFilterValue(value TrackFilterValue) TrackFilterValue {
	switch value.Kind {
	case TrackFilterValueText:
		return TextFilterValue(value.Text)
	case TrackFilterValueUUID:
		return UUIDFilterValue(value.UUID)
	case TrackFilterValueUUIDList:
		return UUIDListFilterValue(value.UUIDs...)
	case TrackFilterValueInteger:
		return IntegerFilterValue(value.Integer)
	case TrackFilterValueIntegerRange:
		return IntegerRangeFilterValue(value.IntegerRange.Minimum, value.IntegerRange.Maximum)
	case TrackFilterValueBoolean:
		return BooleanFilterValue(value.Boolean)
	default:
		return TrackFilterValue{Kind: value.Kind}
	}
}

func filterValueKindError(path string, expected TrackFilterValueKind) error {
	return validationError(path+".value", "must use "+string(expected)+" value")
}

func validateTrackIDs(ids []uuid.UUID) error {
	return validateIDBatch("track_ids", ids)
}

func validateIDBatch(field string, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return validationError(field, "must contain at least one id")
	}
	if len(ids) > maximumTrackIDBatchSize {
		return validationError(field, "must not contain more than 100 ids")
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return validationError(field, "contains an empty id")
		}
		if _, exists := seen[id]; exists {
			return validationError(field, "contains a duplicate id")
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

func validateTrackLyrics(input TrackLyricsInput) (TrackLyricsInput, error) {
	if err := validateID("id", input.ID); err != nil {
		return TrackLyricsInput{}, err
	}
	if err := validateID("track_id", input.TrackID); err != nil {
		return TrackLyricsInput{}, err
	}
	if !input.Type.valid() {
		return TrackLyricsInput{}, validationError("lyrics_type", "must be plain or synchronized")
	}
	input.Language = strings.ToLower(strings.TrimSpace(input.Language))
	if len(input.Language) > 35 || !languageTagPattern.MatchString(input.Language) {
		return TrackLyricsInput{}, validationError("language", "must be a valid language tag")
	}
	content, err := requiredText("content", input.Content, 100000)
	if err != nil {
		return TrackLyricsInput{}, err
	}
	if !input.SourceType.valid() {
		return TrackLyricsInput{}, validationError("source_type", "is unsupported")
	}
	sourceName, err := requiredText("source_name", input.SourceName, 200)
	if err != nil {
		return TrackLyricsInput{}, err
	}
	input.CopyrightNotice = strings.TrimSpace(input.CopyrightNotice)
	if utf8.RuneCountInString(input.CopyrightNotice) > 1000 {
		return TrackLyricsInput{}, validationError("copyright_notice", "must not exceed 1000 characters")
	}
	if input.Published && !input.RightsAuthorized {
		return TrackLyricsInput{}, validationError(
			"published", "requires separately authorized lyrics rights",
		)
	}
	input.Content = content
	input.SourceName = sourceName
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

func (value LyricsType) valid() bool {
	return value == LyricsTypePlain || value == LyricsTypeSynchronized
}

func (value LyricsSourceType) valid() bool {
	switch value {
	case LyricsSourceArtist, LyricsSourcePublisher,
		LyricsSourceLicensedProvider, LyricsSourcePublicDomain:
		return true
	default:
		return false
	}
}
