package release

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maximumTitleLength = 200

func newDraft(input CreateDraftInput) (Draft, error) {
	if input.CreatorID == uuid.Nil {
		return Draft{}, validationError("creator_id", "must be a UUID")
	}
	title, err := validTitle("title", input.Title)
	if err != nil {
		return Draft{}, err
	}
	releaseType, err := validType(input.Type)
	if err != nil {
		return Draft{}, err
	}
	releaseDate, err := validDate(input.ReleaseDate)
	if err != nil {
		return Draft{}, err
	}
	if len(input.Tracks) == 0 {
		return Draft{}, validationError("tracks", "must contain at least one track")
	}
	if releaseType == TypeSingle && len(input.Tracks) != 1 {
		return Draft{}, validationError("tracks", "single must contain exactly one track")
	}

	releaseID := uuid.New()
	tracks := make([]Track, 0, len(input.Tracks))
	positions := make(map[string]struct{}, len(input.Tracks))
	for index, inputTrack := range input.Tracks {
		trackTitle, validationErr := validTitle(
			fmt.Sprintf("tracks[%d].title", index), inputTrack.Title,
		)
		if validationErr != nil {
			return Draft{}, validationErr
		}
		if inputTrack.TrackNumber < 1 {
			return Draft{}, validationError(
				fmt.Sprintf("tracks[%d].track_number", index), "must be positive",
			)
		}
		if inputTrack.DiscNumber < 1 {
			return Draft{}, validationError(
				fmt.Sprintf("tracks[%d].disc_number", index), "must be positive",
			)
		}
		position := fmt.Sprintf("%d:%d", inputTrack.DiscNumber, inputTrack.TrackNumber)
		if _, exists := positions[position]; exists {
			return Draft{}, validationError("tracks", "must not repeat a disc and track position")
		}
		positions[position] = struct{}{}
		tracks = append(tracks, Track{
			ID: uuid.New(), ReleaseID: releaseID, Title: trackTitle,
			TrackNumber: inputTrack.TrackNumber, DiscNumber: inputTrack.DiscNumber,
			Explicit: inputTrack.Explicit,
		})
	}
	if releaseType == TypeSingle && (tracks[0].DiscNumber != 1 || tracks[0].TrackNumber != 1) {
		return Draft{}, validationError("tracks", "single track position must be disc 1, track 1")
	}
	return Draft{
		Release: Release{
			ID: releaseID, CreatorID: input.CreatorID, Title: title,
			Type: releaseType, ReleaseDate: releaseDate, Status: StatusDraft,
		},
		Tracks: tracks,
	}, nil
}

func validTitle(field, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", validationError(field, "must not be blank")
	}
	if !utf8.ValidString(trimmed) || utf8.RuneCountInString(trimmed) > maximumTitleLength {
		return "", validationError(field, "must contain at most 200 characters")
	}
	return trimmed, nil
}

func validType(value Type) (Type, error) {
	switch value {
	case TypeSingle, TypeEP, TypeAlbum, TypeCompilation:
		return value, nil
	default:
		return "", validationError("release_type", "must be single, ep, album, or compilation")
	}
}

func validDate(value time.Time) (time.Time, error) {
	if value.IsZero() {
		return time.Time{}, validationError("release_date", "is required")
	}
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), nil
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
