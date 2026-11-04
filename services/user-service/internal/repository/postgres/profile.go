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
