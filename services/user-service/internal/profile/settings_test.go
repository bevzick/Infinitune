package profile

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestServiceUpdatesOwnedSettings(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := newProfileStore()
	store.profiles[userID] = testProfile(userID)
	service := NewService(store)

	updated, err := service.UpdateSettings(context.Background(), userID, userID, SettingsUpdateInput{
		Theme:                 pointer(ThemeDark),
		Autoplay:              pointer(false),
		ExplicitContent:       pointer(false),
		AudioQuality:          pointer(AudioQualityLossless),
		NormalizeVolume:       pointer(false),
		ShowListeningActivity: pointer(true),
	})
	if err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if updated.Theme != ThemeDark || updated.AudioQuality != AudioQualityLossless {
		t.Errorf("UpdateSettings() theme/quality = %q, %q", updated.Theme, updated.AudioQuality)
	}
	if updated.Autoplay || updated.ExplicitContent || updated.NormalizeVolume {
		t.Errorf("UpdateSettings() boolean playback settings = %+v", updated)
	}
	if !updated.ShowListeningActivity {
		t.Error("UpdateSettings().ShowListeningActivity = false, want true")
	}
	if store.settingsUpdateCalls != 1 {
		t.Fatalf("settings update calls = %d, want 1", store.settingsUpdateCalls)
	}
}

func TestServiceRejectsUpdatingAnotherUsersSettings(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	profileUserID := uuid.New()
	store := newProfileStore()
	store.profiles[profileUserID] = testProfile(profileUserID)
	service := NewService(store)

	_, err := service.UpdateSettings(
		context.Background(),
		actorUserID,
		profileUserID,
		SettingsUpdateInput{Theme: pointer(ThemeDark)},
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("UpdateSettings() error = %v, want %v", err, ErrForbidden)
	}
	if store.readCalls != 0 || store.settingsUpdateCalls != 0 {
		t.Fatalf(
			"store calls = read %d, settings update %d; want none",
			store.readCalls,
			store.settingsUpdateCalls,
		)
	}
}

func TestServiceRejectsInvalidSettings(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	tests := []struct {
		name  string
		input SettingsUpdateInput
		field string
	}{
		{
			name:  "invalid theme",
			input: SettingsUpdateInput{Theme: pointer(InterfaceTheme("neon"))},
			field: "theme",
		},
		{
			name:  "invalid audio quality",
			input: SettingsUpdateInput{AudioQuality: pointer(AudioQuality("studio"))},
			field: "audio_quality",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newProfileStore()
			store.profiles[userID] = testProfile(userID)
			service := NewService(store)

			_, err := service.UpdateSettings(context.Background(), userID, userID, test.input)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("UpdateSettings() error = %v, want %v", err, ErrInvalid)
			}
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("validation error = %#v, want field %q", validationError, test.field)
			}
			if store.settingsUpdateCalls != 0 {
				t.Fatalf("settings update calls = %d, want 0", store.settingsUpdateCalls)
			}
		})
	}
}

func TestServiceSkipsUnchangedSettingsUpdate(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := newProfileStore()
	store.profiles[userID] = testProfile(userID)
	service := NewService(store)

	settings, err := service.UpdateSettings(
		context.Background(),
		userID,
		userID,
		SettingsUpdateInput{},
	)
	if err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if settings.Theme != ThemeSystem || settings.AudioQuality != AudioQualityHigh {
		t.Fatalf("UpdateSettings() = %+v", settings)
	}
	if store.settingsUpdateCalls != 0 {
		t.Fatalf("settings update calls = %d, want 0", store.settingsUpdateCalls)
	}
}
