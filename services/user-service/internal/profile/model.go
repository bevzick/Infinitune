package profile

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("profile not found")
	ErrAlreadyExists = errors.New("profile already exists")
	ErrUsernameTaken = errors.New("username is already taken")
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

type Preferences struct {
	RecommendationsEnabled bool
	PreferredMoodID        *uuid.UUID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type CreateInput struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
}
