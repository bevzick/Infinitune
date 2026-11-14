package rights

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/language"
)

var territoryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

func validateSetInput(input SetInput) (SetInput, error) {
	if input.ReleaseTrackID == uuid.Nil {
		return SetInput{}, validationError("release_track_id", "must be a UUID")
	}
	territory := strings.ToUpper(strings.TrimSpace(input.TerritoryCode))
	if territory != TerritoryWorld && !validTerritory(territory) {
		return SetInput{}, validationError(
			"territory_code", "must be an ISO alpha-2 code or 001 for worldwide",
		)
	}
	if !validType(input.Type) {
		return SetInput{}, validationError(
			"rights_type", "must be streaming, download, or distribution",
		)
	}
	if input.ValidFrom.IsZero() {
		return SetInput{}, validationError("valid_from", "is required")
	}
	input.TerritoryCode = territory
	input.ValidFrom = dateOnly(input.ValidFrom)
	if input.ValidUntil != nil {
		validUntil := dateOnly(*input.ValidUntil)
		if validUntil.Before(input.ValidFrom) {
			return SetInput{}, validationError(
				"valid_until", "must be on or after valid_from",
			)
		}
		input.ValidUntil = &validUntil
	}
	return input, nil
}

func validTerritory(value string) bool {
	if !territoryPattern.MatchString(value) {
		return false
	}
	region, err := language.ParseRegion(value)
	return err == nil && region.IsCountry() && !region.IsPrivateUse()
}

func validType(value Type) bool {
	switch value {
	case TypeStreaming, TypeDownload, TypeDistribution:
		return true
	default:
		return false
	}
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
