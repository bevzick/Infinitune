package profile

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestPreferenceServiceUpdatesOwnedPreferences(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	moodID := uuid.New()
	firstGenreID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	secondGenreID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	artistID := uuid.New()
	store := newPreferenceStore(userID)
	references := &preferenceReferencesStub{}
	service := NewPreferenceService(store, references)

	updated, err := service.Update(context.Background(), userID, userID, PreferencesUpdateInput{
		RecommendationsEnabled: pointer(false),
		PreferredMoodID:        UUIDUpdate{Set: true, Value: &moodID},
		Genres: PreferenceSetUpdate{Set: true, Values: []WeightedPreference{
			{ID: firstGenreID, Weight: 0.25},
			{ID: secondGenreID, Weight: 1},
		}},
		Artists: PreferenceSetUpdate{Set: true, Values: []WeightedPreference{
			{ID: artistID, Weight: 0.875},
		}},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.RecommendationsEnabled || updated.PreferredMoodID == nil ||
		*updated.PreferredMoodID != moodID {
		t.Errorf("Update() base preferences = %+v", updated)
	}
	if len(updated.Genres) != 2 || updated.Genres[0].ID != secondGenreID ||
		updated.Genres[1].ID != firstGenreID {
		t.Errorf("Update().Genres = %+v, want stable UUID order", updated.Genres)
	}
	if len(updated.Artists) != 1 || updated.Artists[0].ID != artistID {
		t.Errorf("Update().Artists = %+v", updated.Artists)
	}
	if !slices.Equal(references.genreIDs, []uuid.UUID{secondGenreID, firstGenreID}) {
		t.Errorf("validated genre ids = %v", references.genreIDs)
	}
	if !slices.Equal(references.artistIDs, []uuid.UUID{artistID}) || references.moodID != moodID {
		t.Errorf("validated artist/mood ids = %v, %s", references.artistIDs, references.moodID)
	}
}

func TestPreferenceServiceClearsOptionalPreferences(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := newPreferenceStore(userID)
	moodID := uuid.New()
	store.preferences[userID] = Preferences{
		RecommendationsEnabled: true,
		PreferredMoodID:        &moodID,
		Genres:                 []WeightedPreference{{ID: uuid.New(), Weight: 1}},
		Artists:                []WeightedPreference{{ID: uuid.New(), Weight: 0.5}},
	}
	service := NewPreferenceService(store, &preferenceReferencesStub{})

	updated, err := service.Update(context.Background(), userID, userID, PreferencesUpdateInput{
		PreferredMoodID: UUIDUpdate{Set: true},
		Genres:          PreferenceSetUpdate{Set: true},
		Artists:         PreferenceSetUpdate{Set: true},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.PreferredMoodID != nil || len(updated.Genres) != 0 || len(updated.Artists) != 0 {
		t.Fatalf("Update(clear) = %+v", updated)
	}
}

func TestPreferenceServiceRejectsAnotherUser(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	profileUserID := uuid.New()
	store := newPreferenceStore(profileUserID)
	service := NewPreferenceService(store, &preferenceReferencesStub{})

	_, err := service.Update(
		context.Background(),
		actorUserID,
		profileUserID,
		PreferencesUpdateInput{RecommendationsEnabled: pointer(false)},
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update() error = %v, want %v", err, ErrForbidden)
	}
	if store.readCalls != 0 || store.updateCalls != 0 {
		t.Fatalf("store calls = read %d, update %d; want none", store.readCalls, store.updateCalls)
	}
}

func TestPreferenceServiceRejectsInvalidWeightedPreferences(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	tests := []struct {
		name   string
		values []WeightedPreference
		field  string
	}{
		{name: "zero id", values: []WeightedPreference{{Weight: 0.5}}, field: "genres[0].id"},
		{
			name:   "duplicate id",
			values: []WeightedPreference{{ID: id, Weight: 0.5}, {ID: id, Weight: 1}},
			field:  "genres[1].id",
		},
		{name: "negative weight", values: []WeightedPreference{{ID: id, Weight: -0.1}}, field: "genres[0].weight"},
		{name: "large weight", values: []WeightedPreference{{ID: id, Weight: 1.1}}, field: "genres[0].weight"},
		{name: "excess precision", values: []WeightedPreference{{ID: id, Weight: 0.1234}}, field: "genres[0].weight"},
		{name: "not a number", values: []WeightedPreference{{ID: id, Weight: math.NaN()}}, field: "genres[0].weight"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := normalizePreferencesUpdate(PreferencesUpdateInput{
				Genres: PreferenceSetUpdate{Set: true, Values: test.values},
			})
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("normalizePreferencesUpdate() error = %v, want %v", err, ErrInvalid)
			}
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("validation error = %#v, want field %q", validationError, test.field)
			}
		})
	}
}

func TestPreferenceServiceRequiresValidReferences(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	referenceError := errors.New("catalog reference not found")
	store := newPreferenceStore(userID)
	service := NewPreferenceService(store, &preferenceReferencesStub{genreError: referenceError})

	_, err := service.Update(context.Background(), userID, userID, PreferencesUpdateInput{
		Genres: PreferenceSetUpdate{Set: true, Values: []WeightedPreference{
			{ID: uuid.New(), Weight: 1},
		}},
	})
	if !errors.Is(err, ErrInvalidReference) || !errors.Is(err, referenceError) {
		t.Fatalf("Update() error = %v, want invalid reference wrapping catalog error", err)
	}
	if store.updateCalls != 0 {
		t.Fatalf("store update calls = %d, want 0", store.updateCalls)
	}
}

type preferenceStore struct {
	preferences map[uuid.UUID]Preferences
	readCalls   int
	updateCalls int
}

func newPreferenceStore(userID uuid.UUID) *preferenceStore {
	return &preferenceStore{preferences: map[uuid.UUID]Preferences{
		userID: {RecommendationsEnabled: true},
	}}
}

func (store *preferenceStore) PreferencesByUserID(
	_ context.Context,
	userID uuid.UUID,
) (Preferences, error) {
	store.readCalls++
	preferences, exists := store.preferences[userID]
	if !exists {
		return Preferences{}, ErrNotFound
	}
	return preferences, nil
}

func (store *preferenceStore) UpdatePreferences(
	_ context.Context,
	userID uuid.UUID,
	input PreferencesUpdateInput,
) (Preferences, error) {
	store.updateCalls++
	preferences := store.preferences[userID]
	if input.RecommendationsEnabled != nil {
		preferences.RecommendationsEnabled = *input.RecommendationsEnabled
	}
	if input.PreferredMoodID.Set {
		preferences.PreferredMoodID = input.PreferredMoodID.Value
	}
	if input.Genres.Set {
		preferences.Genres = append([]WeightedPreference(nil), input.Genres.Values...)
	}
	if input.Artists.Set {
		preferences.Artists = append([]WeightedPreference(nil), input.Artists.Values...)
	}
	store.preferences[userID] = preferences
	return preferences, nil
}

type preferenceReferencesStub struct {
	genreIDs    []uuid.UUID
	artistIDs   []uuid.UUID
	moodID      uuid.UUID
	genreError  error
	artistError error
	moodError   error
}

func (stub *preferenceReferencesStub) ValidateGenreIDs(_ context.Context, ids []uuid.UUID) error {
	stub.genreIDs = append([]uuid.UUID(nil), ids...)
	return stub.genreError
}

func (stub *preferenceReferencesStub) ValidateArtistIDs(_ context.Context, ids []uuid.UUID) error {
	stub.artistIDs = append([]uuid.UUID(nil), ids...)
	return stub.artistError
}

func (stub *preferenceReferencesStub) ValidateMoodID(_ context.Context, id uuid.UUID) error {
	stub.moodID = id
	return stub.moodError
}
