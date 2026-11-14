package profile

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/google/uuid"
)

const weightPrecision = 1000

type PreferenceStore interface {
	PreferencesByUserID(context.Context, uuid.UUID) (Preferences, error)
	UpdatePreferences(context.Context, uuid.UUID, PreferencesUpdateInput) (Preferences, error)
}

type PreferenceReferences interface {
	ValidateGenreIDs(context.Context, []uuid.UUID) error
	ValidateArtistIDs(context.Context, []uuid.UUID) error
	ValidateMoodID(context.Context, uuid.UUID) error
}

type PreferenceService struct {
	store      PreferenceStore
	references PreferenceReferences
}

func NewPreferenceService(
	store PreferenceStore,
	references PreferenceReferences,
) *PreferenceService {
	return &PreferenceService{store: store, references: references}
}

func (service *PreferenceService) Own(
	ctx context.Context,
	actorUserID uuid.UUID,
	profileUserID uuid.UUID,
) (Preferences, error) {
	if !ownsProfile(actorUserID, profileUserID) {
		return Preferences{}, ErrForbidden
	}

	result, err := service.store.PreferencesByUserID(ctx, profileUserID)
	if err != nil {
		return Preferences{}, fmt.Errorf("read own preferences: %w", err)
	}

	return result, nil
}

func (service *PreferenceService) Update(
	ctx context.Context,
	actorUserID uuid.UUID,
	profileUserID uuid.UUID,
	input PreferencesUpdateInput,
) (Preferences, error) {
	if !ownsProfile(actorUserID, profileUserID) {
		return Preferences{}, ErrForbidden
	}

	normalized, changed, err := normalizePreferencesUpdate(input)
	if err != nil {
		return Preferences{}, err
	}
	if !changed {
		return service.Own(ctx, actorUserID, profileUserID)
	}
	if err = service.validateReferences(ctx, normalized); err != nil {
		return Preferences{}, err
	}

	result, err := service.store.UpdatePreferences(ctx, profileUserID, normalized)
	if err != nil {
		return Preferences{}, fmt.Errorf("update preferences: %w", err)
	}

	return result, nil
}

func (service *PreferenceService) validateReferences(
	ctx context.Context,
	input PreferencesUpdateInput,
) error {
	if input.PreferredMoodID.Set && input.PreferredMoodID.Value != nil {
		if err := service.references.ValidateMoodID(ctx, *input.PreferredMoodID.Value); err != nil {
			return fmt.Errorf("%w: validate mood id: %w", ErrInvalidReference, err)
		}
	}
	if input.Genres.Set && len(input.Genres.Values) > 0 {
		if err := service.references.ValidateGenreIDs(ctx, preferenceIDs(input.Genres.Values)); err != nil {
			return fmt.Errorf("%w: validate genre ids: %w", ErrInvalidReference, err)
		}
	}
	if input.Artists.Set && len(input.Artists.Values) > 0 {
		if err := service.references.ValidateArtistIDs(ctx, preferenceIDs(input.Artists.Values)); err != nil {
			return fmt.Errorf("%w: validate artist ids: %w", ErrInvalidReference, err)
		}
	}

	return nil
}

func normalizePreferencesUpdate(
	input PreferencesUpdateInput,
) (PreferencesUpdateInput, bool, error) {
	changed := input.RecommendationsEnabled != nil || input.PreferredMoodID.Set ||
		input.Genres.Set || input.Artists.Set
	if !changed {
		return input, false, nil
	}

	if input.PreferredMoodID.Set && input.PreferredMoodID.Value != nil &&
		*input.PreferredMoodID.Value == uuid.Nil {
		return PreferencesUpdateInput{}, false, invalid("preferred_mood_id", "must be a non-zero UUID")
	}

	genres, err := normalizeWeightedPreferences("genres", input.Genres)
	if err != nil {
		return PreferencesUpdateInput{}, false, err
	}
	artists, err := normalizeWeightedPreferences("artists", input.Artists)
	if err != nil {
		return PreferencesUpdateInput{}, false, err
	}
	input.Genres = genres
	input.Artists = artists

	return input, true, nil
}

func normalizeWeightedPreferences(
	field string,
	update PreferenceSetUpdate,
) (PreferenceSetUpdate, error) {
	if !update.Set {
		return update, nil
	}

	seen := make(map[uuid.UUID]struct{}, len(update.Values))
	values := append([]WeightedPreference(nil), update.Values...)
	for index, preference := range values {
		if preference.ID == uuid.Nil {
			return PreferenceSetUpdate{}, invalid(
				fmt.Sprintf("%s[%d].id", field, index),
				"must be a non-zero UUID",
			)
		}
		if _, exists := seen[preference.ID]; exists {
			return PreferenceSetUpdate{}, invalid(
				fmt.Sprintf("%s[%d].id", field, index),
				"must not be duplicated",
			)
		}
		seen[preference.ID] = struct{}{}

		scaledWeight := preference.Weight * weightPrecision
		if math.IsNaN(preference.Weight) || math.IsInf(preference.Weight, 0) ||
			preference.Weight < 0 || preference.Weight > 1 ||
			math.Abs(scaledWeight-math.Round(scaledWeight)) > 1e-9 {
			return PreferenceSetUpdate{}, invalid(
				fmt.Sprintf("%s[%d].weight", field, index),
				"must be from 0.000 to 1.000 with at most three decimal places",
			)
		}
	}

	slices.SortFunc(values, func(first, second WeightedPreference) int {
		return bytes.Compare(first.ID[:], second.ID[:])
	})
	update.Values = values

	return update, nil
}

func preferenceIDs(preferences []WeightedPreference) []uuid.UUID {
	ids := make([]uuid.UUID, len(preferences))
	for index, preference := range preferences {
		ids[index] = preference.ID
	}
	return ids
}
