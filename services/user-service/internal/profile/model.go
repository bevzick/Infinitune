package profile

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound         = errors.New("profile not found")
	ErrAlreadyExists    = errors.New("profile already exists")
	ErrUsernameTaken    = errors.New("username is already taken")
	ErrForbidden        = errors.New("operation is forbidden")
	ErrInvalid          = errors.New("profile data is invalid")
	ErrInvalidReference = errors.New("preference references are invalid")
)

type InterfaceTheme string

const (
	ThemeLight  InterfaceTheme = "light"
	ThemeDark   InterfaceTheme = "dark"
	ThemeSystem InterfaceTheme = "system"
)

type AudioQuality string

const (
	AudioQualityLow      AudioQuality = "low"
	AudioQualityMedium   AudioQuality = "medium"
	AudioQualityHigh     AudioQuality = "high"
	AudioQualityLossless AudioQuality = "lossless"
)

type Profile struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
	Bio         string
	AvatarURL   string
	CountryCode string
	Language    string
	BirthDate   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Settings    Settings
	Preferences Preferences
}

type Settings struct {
	Theme                 InterfaceTheme
	Autoplay              bool
	ExplicitContent       bool
	AudioQuality          AudioQuality
	NormalizeVolume       bool
	ShowListeningActivity bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type SettingsUpdateInput struct {
	Theme                 *InterfaceTheme
	Autoplay              *bool
	ExplicitContent       *bool
	AudioQuality          *AudioQuality
	NormalizeVolume       *bool
	ShowListeningActivity *bool
}

type Preferences struct {
	RecommendationsEnabled bool
	PreferredMoodID        *uuid.UUID
	Genres                 []WeightedPreference
	Artists                []WeightedPreference
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type WeightedPreference struct {
	ID     uuid.UUID
	Weight float64
}

type PreferenceSetUpdate struct {
	Set    bool
	Values []WeightedPreference
}

type UUIDUpdate struct {
	Set   bool
	Value *uuid.UUID
}

type PreferencesUpdateInput struct {
	RecommendationsEnabled *bool
	PreferredMoodID        UUIDUpdate
	Genres                 PreferenceSetUpdate
	Artists                PreferenceSetUpdate
}

type CreateInput struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
}

type BirthDateUpdate struct {
	Set   bool
	Value *time.Time
}

type UpdateInput struct {
	Username    *string
	DisplayName *string
	Bio         *string
	AvatarURL   *string
	CountryCode *string
	Language    *string
	BirthDate   BirthDateUpdate
}

type PublicProfile struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
	Bio         string
	AvatarURL   string
	CountryCode string
	Language    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string {
	return err.Field + ": " + err.Reason
}

func (err *ValidationError) Unwrap() error {
	return ErrInvalid
}
