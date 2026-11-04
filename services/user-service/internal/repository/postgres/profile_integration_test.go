//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/user-service/internal/database"
	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProfileRepositoryCreateAndRead(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewProfileRepository(pool)
	userID := uuid.New()
	username := "integration_" + userID.String()

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_artist_preferences WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_genre_preferences WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_preferences WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_settings WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".profiles WHERE user_id = $1`, userID)
	})

	created, err := repository.Create(context.Background(), profile.CreateInput{
		UserID:      userID,
		Username:    username,
		DisplayName: "Integration User",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.UserID != userID || created.Username != username {
		t.Fatalf("Create() = %+v, want user %s and username %q", created, userID, username)
	}
	if created.Settings.Theme != profile.ThemeSystem {
		t.Errorf("Create().Settings.Theme = %q, want %q", created.Settings.Theme, profile.ThemeSystem)
	}
	if !created.Preferences.RecommendationsEnabled {
		t.Error("Create().Preferences.RecommendationsEnabled = false, want true")
	}

	read, err := repository.ByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ByUserID() error = %v", err)
	}
	if !reflect.DeepEqual(read, created) {
		t.Fatalf("ByUserID() = %+v, want %+v", read, created)
	}

	created.DisplayName = "Updated Integration User"
	created.Bio = "Repository integration test"
	created.CountryCode = "UA"
	created.Language = "uk-UA"
	birthDate := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
	created.BirthDate = &birthDate
	updated, err := repository.Update(context.Background(), created)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.DisplayName != created.DisplayName || updated.Bio != created.Bio ||
		updated.CountryCode != created.CountryCode || updated.Language != created.Language ||
		updated.BirthDate == nil || !updated.BirthDate.Equal(birthDate) {
		t.Fatalf("Update() = %+v, want editable fields from %+v", updated, created)
	}

	updatedSettings, err := repository.UpdateSettings(context.Background(), userID, profile.Settings{
		Theme:                 profile.ThemeDark,
		Autoplay:              false,
		ExplicitContent:       false,
		AudioQuality:          profile.AudioQualityLossless,
		NormalizeVolume:       false,
		ShowListeningActivity: true,
	})
	if err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if updatedSettings.Theme != profile.ThemeDark ||
		updatedSettings.AudioQuality != profile.AudioQualityLossless ||
		updatedSettings.Autoplay || updatedSettings.ExplicitContent ||
		updatedSettings.NormalizeVolume || !updatedSettings.ShowListeningActivity {
		t.Fatalf("UpdateSettings() = %+v", updatedSettings)
	}

	readAfterSettingsUpdate, err := repository.ByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ByUserID(after settings update) error = %v", err)
	}
	if readAfterSettingsUpdate.Settings != updatedSettings {
		t.Fatalf(
			"ByUserID().Settings = %+v, want %+v",
			readAfterSettingsUpdate.Settings,
			updatedSettings,
		)
	}

	moodID := uuid.New()
	genreID := uuid.New()
	artistID := uuid.New()
	updatedPreferences, err := repository.UpdatePreferences(
		context.Background(),
		userID,
		profile.PreferencesUpdateInput{
			RecommendationsEnabled: boolPointer(false),
			PreferredMoodID:        profile.UUIDUpdate{Set: true, Value: &moodID},
			Genres: profile.PreferenceSetUpdate{Set: true, Values: []profile.WeightedPreference{
				{ID: genreID, Weight: 0.625},
			}},
			Artists: profile.PreferenceSetUpdate{Set: true, Values: []profile.WeightedPreference{
				{ID: artistID, Weight: 1},
			}},
		},
	)
	if err != nil {
		t.Fatalf("UpdatePreferences() error = %v", err)
	}
	if updatedPreferences.RecommendationsEnabled || updatedPreferences.PreferredMoodID == nil ||
		*updatedPreferences.PreferredMoodID != moodID ||
		len(updatedPreferences.Genres) != 1 || updatedPreferences.Genres[0].ID != genreID ||
		updatedPreferences.Genres[0].Weight != 0.625 ||
		len(updatedPreferences.Artists) != 1 || updatedPreferences.Artists[0].ID != artistID ||
		updatedPreferences.Artists[0].Weight != 1 {
		t.Fatalf("UpdatePreferences() = %+v", updatedPreferences)
	}

	clearedPreferences, err := repository.UpdatePreferences(
		context.Background(),
		userID,
		profile.PreferencesUpdateInput{
			PreferredMoodID: profile.UUIDUpdate{Set: true},
			Genres:          profile.PreferenceSetUpdate{Set: true},
			Artists:         profile.PreferenceSetUpdate{Set: true},
		},
	)
	if err != nil {
		t.Fatalf("UpdatePreferences(clear) error = %v", err)
	}
	if clearedPreferences.PreferredMoodID != nil ||
		len(clearedPreferences.Genres) != 0 || len(clearedPreferences.Artists) != 0 {
		t.Fatalf("UpdatePreferences(clear) = %+v", clearedPreferences)
	}
}

func TestProfileRepositoryRejectsDuplicateUsername(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewProfileRepository(pool)
	firstUserID := uuid.New()
	secondUserID := uuid.New()
	username := "integration_" + firstUserID.String()

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, userID := range []uuid.UUID{firstUserID, secondUserID} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_artist_preferences WHERE user_id = $1`, userID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_genre_preferences WHERE user_id = $1`, userID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_preferences WHERE user_id = $1`, userID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".user_settings WHERE user_id = $1`, userID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user".profiles WHERE user_id = $1`, userID)
		}
	})

	_, err := repository.Create(context.Background(), profile.CreateInput{
		UserID:      firstUserID,
		Username:    username,
		DisplayName: "First User",
	})
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	_, err = repository.Create(context.Background(), profile.CreateInput{
		UserID:      secondUserID,
		Username:    strings.ToUpper(username),
		DisplayName: "Second User",
	})
	if !errors.Is(err, profile.ErrUsernameTaken) {
		t.Fatalf("second Create() error = %v, want %v", err, profile.ErrUsernameTaken)
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func TestProfileRepositoryReturnsNotFound(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewProfileRepository(pool)

	_, err := repository.ByUserID(context.Background(), uuid.New())
	if !errors.Is(err, profile.ErrNotFound) {
		t.Fatalf("ByUserID() error = %v, want %v", err, profile.ErrNotFound)
	}
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}

	port, err := strconv.ParseUint(envOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatalf("parse POSTGRES_PORT: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, database.Config{
		Host:     envOrDefault("POSTGRES_HOST", "127.0.0.1"),
		Port:     uint16(port),
		User:     requiredEnvironment(t, "POSTGRES_USER"),
		Password: requiredEnvironment(t, "POSTGRES_PASSWORD"),
		Name:     requiredEnvironment(t, "POSTGRES_USER_DB"),
	})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func requiredEnvironment(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
