package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type profileDatabase interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type ProfileRepository struct {
	database profileDatabase
}

func NewProfileRepository(database profileDatabase) *ProfileRepository {
	return &ProfileRepository{database: database}
}

func (repository *ProfileRepository) Create(
	ctx context.Context,
	input profile.CreateInput,
) (profile.Profile, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("begin create profile transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO "user".profiles (user_id, username, display_name)
		VALUES ($1, $2, $3)
	`, input.UserID, input.Username, input.DisplayName)
	if err != nil {
		if mappedError := mapConstraintError(err); mappedError != nil {
			return profile.Profile{}, mappedError
		}
		return profile.Profile{}, fmt.Errorf("insert profile: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO "user".user_settings (user_id)
		VALUES ($1)
	`, input.UserID); err != nil {
		return profile.Profile{}, fmt.Errorf("insert user settings: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO "user".user_preferences (user_id)
		VALUES ($1)
	`, input.UserID); err != nil {
		return profile.Profile{}, fmt.Errorf("insert user preferences: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return profile.Profile{}, fmt.Errorf("commit create profile transaction: %w", err)
	}

	created, err := repository.ByUserID(ctx, input.UserID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("read created profile: %w", err)
	}

	return created, nil
}

func (repository *ProfileRepository) ByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (profile.Profile, error) {
	var result profile.Profile
	err := repository.database.QueryRow(ctx, `
		SELECT
			p.user_id,
			p.username,
			p.display_name,
			p.bio,
			p.avatar_url,
			p.country_code,
			p.language,
			p.birth_date,
			p.created_at,
			p.updated_at,
			s.theme,
			s.autoplay,
			s.explicit_content,
			s.audio_quality,
			s.normalize_volume,
			s.show_listening_activity,
			s.created_at,
			s.updated_at,
			pr.recommendations_enabled,
			pr.preferred_mood_id,
			pr.created_at,
			pr.updated_at
		FROM "user".profiles AS p
		JOIN "user".user_settings AS s ON s.user_id = p.user_id
		JOIN "user".user_preferences AS pr ON pr.user_id = p.user_id
		WHERE p.user_id = $1
	`, userID).Scan(
		&result.UserID,
		&result.Username,
		&result.DisplayName,
		&result.Bio,
		&result.AvatarURL,
		&result.CountryCode,
		&result.Language,
		&result.BirthDate,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.Settings.Theme,
		&result.Settings.Autoplay,
		&result.Settings.ExplicitContent,
		&result.Settings.AudioQuality,
		&result.Settings.NormalizeVolume,
		&result.Settings.ShowListeningActivity,
		&result.Settings.CreatedAt,
		&result.Settings.UpdatedAt,
		&result.Preferences.RecommendationsEnabled,
		&result.Preferences.PreferredMoodID,
		&result.Preferences.CreatedAt,
		&result.Preferences.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.Profile{}, profile.ErrNotFound
	}
	if err != nil {
		return profile.Profile{}, fmt.Errorf("query profile by user id: %w", err)
	}
	result.Preferences, err = repository.PreferencesByUserID(ctx, userID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("query complete profile preferences: %w", err)
	}

	return result, nil
}

func (repository *ProfileRepository) Update(
	ctx context.Context,
	updated profile.Profile,
) (profile.Profile, error) {
	commandTag, err := repository.database.Exec(ctx, `
		UPDATE "user".profiles
		SET
			username = $2,
			display_name = $3,
			bio = $4,
			avatar_url = $5,
			country_code = $6,
			language = $7,
			birth_date = $8,
			updated_at = now()
		WHERE user_id = $1
	`,
		updated.UserID,
		updated.Username,
		updated.DisplayName,
		updated.Bio,
		updated.AvatarURL,
		updated.CountryCode,
		updated.Language,
		updated.BirthDate,
	)
	if err != nil {
		if mappedError := mapConstraintError(err); mappedError != nil {
			return profile.Profile{}, mappedError
		}
		return profile.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return profile.Profile{}, profile.ErrNotFound
	}

	result, err := repository.ByUserID(ctx, updated.UserID)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("read updated profile: %w", err)
	}

	return result, nil
}

func (repository *ProfileRepository) UpdateSettings(
	ctx context.Context,
	userID uuid.UUID,
	settings profile.Settings,
) (profile.Settings, error) {
	var result profile.Settings
	err := repository.database.QueryRow(ctx, `
		UPDATE "user".user_settings
		SET
			theme = $2,
			autoplay = $3,
			explicit_content = $4,
			audio_quality = $5,
			normalize_volume = $6,
			show_listening_activity = $7,
			updated_at = now()
		WHERE user_id = $1
		RETURNING
			theme,
			autoplay,
			explicit_content,
			audio_quality,
			normalize_volume,
			show_listening_activity,
			created_at,
			updated_at
	`,
		userID,
		settings.Theme,
		settings.Autoplay,
		settings.ExplicitContent,
		settings.AudioQuality,
		settings.NormalizeVolume,
		settings.ShowListeningActivity,
	).Scan(
		&result.Theme,
		&result.Autoplay,
		&result.ExplicitContent,
		&result.AudioQuality,
		&result.NormalizeVolume,
		&result.ShowListeningActivity,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.Settings{}, profile.ErrNotFound
	}
	if err != nil {
		return profile.Settings{}, fmt.Errorf("update user settings: %w", err)
	}

	return result, nil
}

func (repository *ProfileRepository) PreferencesByUserID(
	ctx context.Context,
	userID uuid.UUID,
) (profile.Preferences, error) {
	var result profile.Preferences
	err := repository.database.QueryRow(ctx, `
		SELECT
			recommendations_enabled,
			preferred_mood_id,
			created_at,
			updated_at
		FROM "user".user_preferences
		WHERE user_id = $1
	`, userID).Scan(
		&result.RecommendationsEnabled,
		&result.PreferredMoodID,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.Preferences{}, profile.ErrNotFound
	}
	if err != nil {
		return profile.Preferences{}, fmt.Errorf("query user preferences: %w", err)
	}

	result.Genres, err = repository.weightedPreferences(ctx, `
		SELECT genre_id, weight
		FROM "user".user_genre_preferences
		WHERE user_id = $1
		ORDER BY genre_id
	`, userID)
	if err != nil {
		return profile.Preferences{}, fmt.Errorf("query genre preferences: %w", err)
	}
	result.Artists, err = repository.weightedPreferences(ctx, `
		SELECT artist_id, weight
		FROM "user".user_artist_preferences
		WHERE user_id = $1
		ORDER BY artist_id
	`, userID)
	if err != nil {
		return profile.Preferences{}, fmt.Errorf("query artist preferences: %w", err)
	}

	return result, nil
}

func (repository *ProfileRepository) UpdatePreferences(
	ctx context.Context,
	userID uuid.UUID,
	input profile.PreferencesUpdateInput,
) (profile.Preferences, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return profile.Preferences{}, fmt.Errorf("begin preferences transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var recommendationsEnabled any
	if input.RecommendationsEnabled != nil {
		recommendationsEnabled = *input.RecommendationsEnabled
	}
	var preferredMoodID any
	if input.PreferredMoodID.Value != nil {
		preferredMoodID = *input.PreferredMoodID.Value
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE "user".user_preferences
		SET
			recommendations_enabled = COALESCE($2::boolean, recommendations_enabled),
			preferred_mood_id = CASE WHEN $3::boolean THEN $4::uuid ELSE preferred_mood_id END,
			updated_at = now()
		WHERE user_id = $1
	`, userID, recommendationsEnabled, input.PreferredMoodID.Set, preferredMoodID)
	if err != nil {
		return profile.Preferences{}, fmt.Errorf("update base preferences: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return profile.Preferences{}, profile.ErrNotFound
	}

	if input.Genres.Set {
		if err = replaceWeightedPreferences(
			ctx,
			tx,
			`DELETE FROM "user".user_genre_preferences WHERE user_id = $1`,
			`INSERT INTO "user".user_genre_preferences (user_id, genre_id, weight) VALUES ($1, $2, $3)`,
			userID,
			input.Genres.Values,
		); err != nil {
			return profile.Preferences{}, fmt.Errorf("replace genre preferences: %w", err)
		}
	}
	if input.Artists.Set {
		if err = replaceWeightedPreferences(
			ctx,
			tx,
			`DELETE FROM "user".user_artist_preferences WHERE user_id = $1`,
			`INSERT INTO "user".user_artist_preferences (user_id, artist_id, weight) VALUES ($1, $2, $3)`,
			userID,
			input.Artists.Values,
		); err != nil {
			return profile.Preferences{}, fmt.Errorf("replace artist preferences: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return profile.Preferences{}, fmt.Errorf("commit preferences transaction: %w", err)
	}

	result, err := repository.PreferencesByUserID(ctx, userID)
	if err != nil {
		return profile.Preferences{}, fmt.Errorf("read updated preferences: %w", err)
	}

	return result, nil
}

func (repository *ProfileRepository) weightedPreferences(
	ctx context.Context,
	query string,
	userID uuid.UUID,
) ([]profile.WeightedPreference, error) {
	rows, err := repository.database.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]profile.WeightedPreference, 0)
	for rows.Next() {
		var preference profile.WeightedPreference
		if err = rows.Scan(&preference.ID, &preference.Weight); err != nil {
			return nil, err
		}
		result = append(result, preference)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func replaceWeightedPreferences(
	ctx context.Context,
	tx pgx.Tx,
	deleteQuery string,
	insertQuery string,
	userID uuid.UUID,
	preferences []profile.WeightedPreference,
) error {
	if _, err := tx.Exec(ctx, deleteQuery, userID); err != nil {
		return err
	}
	for _, preference := range preferences {
		if _, err := tx.Exec(ctx, insertQuery, userID, preference.ID, preference.Weight); err != nil {
			return err
		}
	}

	return nil
}

func mapConstraintError(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23505" {
		return nil
	}

	switch postgresError.ConstraintName {
	case "profiles_username_key":
		return profile.ErrUsernameTaken
	case "profiles_pkey":
		return profile.ErrAlreadyExists
	default:
		return nil
	}
}
