package profile

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceCreatesOwnedProfileWithCanonicalValues(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := newProfileStore()
	service := NewService(store)

	created, err := service.Create(context.Background(), userID, CreateInput{
		UserID:      userID,
		Username:    "  New_User  ",
		DisplayName: "  New User  ",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Username != "new_user" {
		t.Errorf("Create().Username = %q, want new_user", created.Username)
	}
	if created.DisplayName != "New User" {
		t.Errorf("Create().DisplayName = %q, want New User", created.DisplayName)
	}
}

func TestServiceRejectsAccessToAnotherUsersPrivateProfile(t *testing.T) {
	t.Parallel()

	actorUserID := uuid.New()
	profileUserID := uuid.New()
	store := newProfileStore()
	store.profiles[profileUserID] = testProfile(profileUserID)
	service := NewService(store)

	if _, err := service.Own(context.Background(), actorUserID, profileUserID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Own() error = %v, want %v", err, ErrForbidden)
	}
	if _, err := service.Update(
		context.Background(),
		actorUserID,
		profileUserID,
		UpdateInput{DisplayName: pointer("Changed")},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update() error = %v, want %v", err, ErrForbidden)
	}
	if store.readCalls != 0 || store.updateCalls != 0 {
		t.Fatalf("store calls = read %d, update %d; want none", store.readCalls, store.updateCalls)
	}
}

func TestServiceUpdatesOwnedProfile(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := newProfileStore()
	store.profiles[userID] = testProfile(userID)
	service := NewService(store)
	service.now = func() time.Time {
		return time.Date(2026, time.November, 4, 12, 0, 0, 0, time.UTC)
	}
	birthDate := time.Date(2000, time.January, 2, 17, 30, 0, 0, time.FixedZone("test", 2*60*60))

	updated, err := service.Update(context.Background(), userID, userID, UpdateInput{
		Username:    pointer(" Updated_User "),
		DisplayName: pointer(" Updated Name "),
		Bio:         pointer(" About the listener. "),
		AvatarURL:   pointer("https://cdn.example.test/avatar.png"),
		CountryCode: pointer("ua"),
		Language:    pointer("uk-ua"),
		BirthDate:   BirthDateUpdate{Set: true, Value: &birthDate},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.Username != "updated_user" || updated.DisplayName != "Updated Name" {
		t.Errorf("Update() names = %q, %q", updated.Username, updated.DisplayName)
	}
	if updated.Bio != "About the listener." || updated.AvatarURL == "" {
		t.Errorf("Update() bio/avatar = %q, %q", updated.Bio, updated.AvatarURL)
	}
	if updated.CountryCode != "UA" || updated.Language != "uk-UA" {
		t.Errorf("Update() locale = %q, %q", updated.CountryCode, updated.Language)
	}
	wantBirthDate := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
	if updated.BirthDate == nil || !updated.BirthDate.Equal(wantBirthDate) {
		t.Errorf("Update().BirthDate = %v, want %v", updated.BirthDate, wantBirthDate)
	}
	if store.updateCalls != 1 {
		t.Fatalf("store update calls = %d, want 1", store.updateCalls)
	}
}

func TestServiceCanClearBirthDateAndSkipUnchangedUpdate(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := newProfileStore()
	current := testProfile(userID)
	birthDate := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
	current.BirthDate = &birthDate
	current.CountryCode = "UA"
	current.Language = "uk-UA"
	store.profiles[userID] = current
	service := NewService(store)

	updated, err := service.Update(context.Background(), userID, userID, UpdateInput{
		CountryCode: pointer(""),
		Language:    pointer(""),
		BirthDate:   BirthDateUpdate{Set: true},
	})
	if err != nil {
		t.Fatalf("Update(clear birth date) error = %v", err)
	}
	if updated.BirthDate != nil {
		t.Fatalf("Update(clear birth date).BirthDate = %v, want nil", updated.BirthDate)
	}
	if updated.CountryCode != "ZZ" || updated.Language != "und" {
		t.Fatalf("Update(clear locale) = %q, %q; want ZZ, und", updated.CountryCode, updated.Language)
	}

	if _, err = service.Update(context.Background(), userID, userID, UpdateInput{}); err != nil {
		t.Fatalf("Update(no changes) error = %v", err)
	}
	if store.updateCalls != 1 {
		t.Fatalf("store update calls = %d, want 1", store.updateCalls)
	}
}

func TestServicePublicProfileExcludesPrivateFields(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	store := newProfileStore()
	store.profiles[userID] = testProfile(userID)
	service := NewService(store)

	public, err := service.Public(context.Background(), userID)
	if err != nil {
		t.Fatalf("Public() error = %v", err)
	}
	if public.UserID != userID || public.Username != "listener" {
		t.Fatalf("Public() = %+v", public)
	}

	publicType := reflect.TypeOf(public)
	for _, privateField := range []string{"BirthDate", "Settings", "Preferences"} {
		if _, exists := publicType.FieldByName(privateField); exists {
			t.Errorf("PublicProfile exposes private field %s", privateField)
		}
	}
}

func TestProfileValidationRejectsInvalidUpdates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 4, 0, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 0, 1)
	tests := []struct {
		name  string
		input UpdateInput
		field string
	}{
		{name: "short username", input: UpdateInput{Username: pointer("ab")}, field: "username"},
		{name: "username punctuation", input: UpdateInput{Username: pointer("not-valid!")}, field: "username"},
		{name: "empty display name", input: UpdateInput{DisplayName: pointer("  ")}, field: "display_name"},
		{name: "long bio", input: UpdateInput{Bio: pointer(strings.Repeat("a", 501))}, field: "bio"},
		{name: "relative avatar", input: UpdateInput{AvatarURL: pointer("/avatar.png")}, field: "avatar_url"},
		{name: "unknown country", input: UpdateInput{CountryCode: pointer("ZZ")}, field: "country_code"},
		{name: "invalid language", input: UpdateInput{Language: pointer("not_a_language")}, field: "language"},
		{name: "future birth date", input: UpdateInput{BirthDate: BirthDateUpdate{Set: true, Value: &future}}, field: "birth_date"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := applyUpdate(testProfile(uuid.New()), test.input, now)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("applyUpdate() error = %v, want %v", err, ErrInvalid)
			}
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("applyUpdate() validation error = %#v, want field %q", validationError, test.field)
			}
		})
	}
}

type profileStore struct {
	profiles    map[uuid.UUID]Profile
	readCalls   int
	updateCalls int
}

func newProfileStore() *profileStore {
	return &profileStore{profiles: make(map[uuid.UUID]Profile)}
}

func (store *profileStore) Create(_ context.Context, input CreateInput) (Profile, error) {
	created := testProfile(input.UserID)
	created.Username = input.Username
	created.DisplayName = input.DisplayName
	store.profiles[input.UserID] = created
	return created, nil
}

func (store *profileStore) ByUserID(_ context.Context, userID uuid.UUID) (Profile, error) {
	store.readCalls++
	result, exists := store.profiles[userID]
	if !exists {
		return Profile{}, ErrNotFound
	}
	return result, nil
}

func (store *profileStore) Update(_ context.Context, updated Profile) (Profile, error) {
	store.updateCalls++
	store.profiles[updated.UserID] = updated
	return updated, nil
}

func testProfile(userID uuid.UUID) Profile {
	return Profile{
		UserID:      userID,
		Username:    "listener",
		DisplayName: "Listener",
		CountryCode: "ZZ",
		Language:    "und",
	}
}

func pointer[T any](value T) *T {
	return &value
}
