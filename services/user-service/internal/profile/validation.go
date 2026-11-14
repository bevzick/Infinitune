package profile

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
)

const (
	minimumUsernameLength  = 3
	maximumUsernameLength  = 50
	maximumDisplayLength   = 100
	maximumBioLength       = 500
	maximumAvatarURLLength = 2048
	maximumLanguageLength  = 10
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

func applyUpdate(current Profile, input UpdateInput, now time.Time) (Profile, bool, error) {
	updated := current
	changed := false

	if input.Username != nil {
		value, err := validateUsername(*input.Username)
		if err != nil {
			return Profile{}, false, err
		}
		if value != updated.Username {
			updated.Username = value
			changed = true
		}
	}
	if input.DisplayName != nil {
		value, err := validateDisplayName(*input.DisplayName)
		if err != nil {
			return Profile{}, false, err
		}
		if value != updated.DisplayName {
			updated.DisplayName = value
			changed = true
		}
	}
	if input.Bio != nil {
		value, err := validateBio(*input.Bio)
		if err != nil {
			return Profile{}, false, err
		}
		if value != updated.Bio {
			updated.Bio = value
			changed = true
		}
	}
	if input.AvatarURL != nil {
		value, err := validateAvatarURL(*input.AvatarURL)
		if err != nil {
			return Profile{}, false, err
		}
		if value != updated.AvatarURL {
			updated.AvatarURL = value
			changed = true
		}
	}
	if input.CountryCode != nil {
		value, err := validateCountryCode(*input.CountryCode)
		if err != nil {
			return Profile{}, false, err
		}
		if value != updated.CountryCode {
			updated.CountryCode = value
			changed = true
		}
	}
	if input.Language != nil {
		value, err := validateLanguage(*input.Language)
		if err != nil {
			return Profile{}, false, err
		}
		if value != updated.Language {
			updated.Language = value
			changed = true
		}
	}
	if input.BirthDate.Set {
		value, err := validateBirthDate(input.BirthDate.Value, now)
		if err != nil {
			return Profile{}, false, err
		}
		if !equalOptionalDate(value, updated.BirthDate) {
			updated.BirthDate = value
			changed = true
		}
	}

	return updated, changed, nil
}

func validateUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < minimumUsernameLength || len(value) > maximumUsernameLength {
		return "", invalid("username", "must contain from 3 to 50 characters")
	}
	if !usernamePattern.MatchString(value) {
		return "", invalid("username", "may contain only lowercase letters, digits, and underscores")
	}
	return value, nil
}

func validateDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	if length == 0 || length > maximumDisplayLength {
		return "", invalid("display_name", "must contain from 1 to 100 characters")
	}
	if containsControl(value, false) {
		return "", invalid("display_name", "must not contain control characters")
	}
	return value, nil
}

func validateBio(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maximumBioLength {
		return "", invalid("bio", "must contain at most 500 characters")
	}
	if containsControl(value, true) {
		return "", invalid("bio", "contains unsupported control characters")
	}
	return value, nil
}

func validateAvatarURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > maximumAvatarURLLength {
		return "", invalid("avatar_url", "must contain at most 2048 bytes")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", invalid("avatar_url", "must be an absolute HTTP or HTTPS URL without credentials")
	}
	return parsed.String(), nil
}

func validateCountryCode(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "ZZ", nil
	}
	region, err := language.ParseRegion(value)
	if err != nil || region.String() == "ZZ" {
		return "", invalid("country_code", "must be an ISO 3166-1 alpha-2 country code")
	}
	return region.String(), nil
}

func validateLanguage(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return language.Und.String(), nil
	}
	tag, err := language.Parse(value)
	if err != nil {
		return "", invalid("language", "must be a valid BCP 47 language tag")
	}
	canonical := tag.String()
	if len(canonical) > maximumLanguageLength {
		return "", invalid("language", "canonical language tag must contain at most 10 bytes")
	}
	return canonical, nil
}

func validateBirthDate(value *time.Time, now time.Time) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	normalized := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if normalized.After(today) {
		return nil, invalid("birth_date", "must not be in the future")
	}
	return &normalized, nil
}

func equalOptionalDate(first, second *time.Time) bool {
	if first == nil || second == nil {
		return first == second
	}
	return first.Equal(*second)
}

func containsControl(value string, allowWhitespace bool) bool {
	for _, character := range value {
		if !unicode.IsControl(character) {
			continue
		}
		if allowWhitespace && (character == '\n' || character == '\r' || character == '\t') {
			continue
		}
		return true
	}
	return false
}

func invalid(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
