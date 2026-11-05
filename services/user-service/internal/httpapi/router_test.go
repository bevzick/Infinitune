package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/user-service/internal/identity"
	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/google/uuid"
)

type healthCheckStub struct {
	err error
}

func (stub healthCheckStub) Ping(context.Context) error {
	return stub.err
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		database   HealthCheck
		wantStatus int
		wantBody   string
	}{
		{
			name:       "database available",
			database:   healthCheckStub{},
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "database unavailable",
			database:   healthCheckStub{err: errors.New("database unavailable")},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"status":"unavailable"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()

			NewRouter(testDependencies(test.database)).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Body.String() != test.wantBody {
				t.Errorf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
		})
	}
}

func TestProtectedRoutesRequireVerifiedBearerToken(t *testing.T) {
	dependencies := testDependencies(healthCheckStub{})
	dependencies.Tokens = identityVerifierStub{err: identity.ErrInvalidAccessToken}
	request := httptest.NewRequest(http.MethodGet, "/v1/users/me/profile", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	request.Header.Set("X-User-ID", uuid.NewString())
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}

func TestProfileRoutesUseTokenIdentityAndKeepPrivateFieldsPrivate(t *testing.T) {
	userID := uuid.New()
	birthDate := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
	profiles := &profilesStub{profile: profile.Profile{
		UserID: userID, Username: "listener", DisplayName: "Listener", BirthDate: &birthDate,
		Settings: profile.Settings{Theme: profile.ThemeSystem, AudioQuality: profile.AudioQualityHigh},
	}}
	dependencies := testDependencies(healthCheckStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: userID}}
	dependencies.Profiles = profiles
	router := NewRouter(dependencies)

	ownRequest := httptest.NewRequest(http.MethodGet, "/v1/users/me/profile", nil)
	ownRequest.Header.Set("Authorization", "Bearer valid")
	ownRequest.Header.Set("X-User-ID", uuid.NewString())
	ownResponse := httptest.NewRecorder()
	router.ServeHTTP(ownResponse, ownRequest)
	if ownResponse.Code != http.StatusOK || profiles.lastActorID != userID {
		t.Fatalf("own profile status = %d, actor = %s, body = %s", ownResponse.Code, profiles.lastActorID, ownResponse.Body.String())
	}
	if !strings.Contains(ownResponse.Body.String(), `"birth_date":"2000-01-02"`) {
		t.Fatalf("own profile body = %s", ownResponse.Body.String())
	}

	publicRequest := httptest.NewRequest(http.MethodGet, "/v1/users/profiles/"+userID.String(), nil)
	publicResponse := httptest.NewRecorder()
	router.ServeHTTP(publicResponse, publicRequest)
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("public profile status = %d, body = %s", publicResponse.Code, publicResponse.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(publicResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode public profile: %v", err)
	}
	for _, privateField := range []string{"birth_date", "settings", "preferences"} {
		if _, exists := body[privateField]; exists {
			t.Errorf("public profile exposes %s: %s", privateField, publicResponse.Body.String())
		}
	}
}

func TestPatchRoutesPreserveNullableAndFalseValues(t *testing.T) {
	userID := uuid.New()
	profiles := &profilesStub{profile: profile.Profile{UserID: userID}}
	dependencies := testDependencies(healthCheckStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: userID}}
	dependencies.Profiles = profiles
	router := NewRouter(dependencies)

	profileRequest := httptest.NewRequest(
		http.MethodPatch,
		"/v1/users/me/profile",
		strings.NewReader(`{"birth_date":null,"display_name":"Updated"}`),
	)
	profileRequest.Header.Set("Authorization", "Bearer valid")
	profileRequest.Header.Set("Content-Type", "application/json")
	profileResponse := httptest.NewRecorder()
	router.ServeHTTP(profileResponse, profileRequest)
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("profile patch status = %d, body = %s", profileResponse.Code, profileResponse.Body.String())
	}
	if !profiles.profileUpdate.BirthDate.Set || profiles.profileUpdate.BirthDate.Value != nil {
		t.Fatalf("birth date update = %+v, want explicit null", profiles.profileUpdate.BirthDate)
	}

	settingsRequest := httptest.NewRequest(
		http.MethodPatch,
		"/v1/users/me/settings",
		strings.NewReader(`{"autoplay":false,"explicit_content":false}`),
	)
	settingsRequest.Header.Set("Authorization", "Bearer valid")
	settingsRequest.Header.Set("Content-Type", "application/json")
	settingsResponse := httptest.NewRecorder()
	router.ServeHTTP(settingsResponse, settingsRequest)
	if settingsResponse.Code != http.StatusOK {
		t.Fatalf("settings patch status = %d, body = %s", settingsResponse.Code, settingsResponse.Body.String())
	}
	if profiles.settingsUpdate.Autoplay == nil || *profiles.settingsUpdate.Autoplay ||
		profiles.settingsUpdate.ExplicitContent == nil || *profiles.settingsUpdate.ExplicitContent {
		t.Fatalf("settings update = %+v, want explicit false values", profiles.settingsUpdate)
	}
}

func TestCreateProfileUsesVerifiedUserID(t *testing.T) {
	userID := uuid.New()
	profiles := &profilesStub{profile: profile.Profile{
		UserID: userID, Username: "listener", DisplayName: "Listener",
	}}
	dependencies := testDependencies(healthCheckStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: userID}}
	dependencies.Profiles = profiles
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/users/me/profile",
		strings.NewReader(`{"username":"listener","display_name":"Listener"}`),
	)
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusCreated || profiles.lastActorID != userID {
		t.Fatalf("status = %d, actor = %s, body = %s", response.Code, profiles.lastActorID, response.Body.String())
	}
}

func TestCORSPreflightDoesNotRequireToken(t *testing.T) {
	dependencies := testDependencies(healthCheckStub{})
	dependencies.Origins = []string{"http://localhost:5173"}
	request := httptest.NewRequest(http.MethodOptions, "/v1/users/me/profile", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func testDependencies(database HealthCheck) Dependencies {
	return Dependencies{
		Database: database,
		Tokens:   identityVerifierStub{identity: identity.Identity{UserID: uuid.New()}},
		Profiles: &profilesStub{},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

type identityVerifierStub struct {
	identity identity.Identity
	err      error
}

func (stub identityVerifierStub) Verify(context.Context, string, time.Time) (identity.Identity, error) {
	return stub.identity, stub.err
}

type profilesStub struct {
	profile        profile.Profile
	lastActorID    uuid.UUID
	profileUpdate  profile.UpdateInput
	settingsUpdate profile.SettingsUpdateInput
}

func (stub *profilesStub) Create(
	_ context.Context,
	actorID uuid.UUID,
	_ profile.CreateInput,
) (profile.Profile, error) {
	stub.lastActorID = actorID
	return stub.profile, nil
}

func (stub *profilesStub) Own(
	_ context.Context,
	actorID uuid.UUID,
	_ uuid.UUID,
) (profile.Profile, error) {
	stub.lastActorID = actorID
	return stub.profile, nil
}

func (stub *profilesStub) Public(context.Context, uuid.UUID) (profile.PublicProfile, error) {
	return profile.PublicProfile{
		UserID: stub.profile.UserID, Username: stub.profile.Username,
		DisplayName: stub.profile.DisplayName, Bio: stub.profile.Bio,
		AvatarURL: stub.profile.AvatarURL, CountryCode: stub.profile.CountryCode,
		Language: stub.profile.Language, CreatedAt: stub.profile.CreatedAt, UpdatedAt: stub.profile.UpdatedAt,
	}, nil
}

func (stub *profilesStub) Update(
	_ context.Context,
	actorID uuid.UUID,
	_ uuid.UUID,
	input profile.UpdateInput,
) (profile.Profile, error) {
	stub.lastActorID = actorID
	stub.profileUpdate = input
	return stub.profile, nil
}

func (stub *profilesStub) UpdateSettings(
	_ context.Context,
	actorID uuid.UUID,
	_ uuid.UUID,
	input profile.SettingsUpdateInput,
) (profile.Settings, error) {
	stub.lastActorID = actorID
	stub.settingsUpdate = input
	return stub.profile.Settings, nil
}
