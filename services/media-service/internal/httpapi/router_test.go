package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/identity"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/google/uuid"
)

type databaseStub struct{ err error }

func (stub databaseStub) Ping(context.Context) error { return stub.err }

type storageStub struct{ err error }

func (stub storageStub) Check(context.Context) error { return stub.err }

func TestHealthChecksDatabaseAndStorage(t *testing.T) {
	tests := []struct {
		name       string
		database   DatabaseCheck
		storage    StorageCheck
		wantStatus int
	}{
		{"available", databaseStub{}, storageStub{}, http.StatusOK},
		{"database unavailable", databaseStub{errors.New("database")}, storageStub{}, http.StatusServiceUnavailable},
		{"storage unavailable", databaseStub{}, storageStub{errors.New("storage")}, http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()
			NewRouter(Dependencies{Database: test.database, Storage: test.storage}).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

type identityVerifierStub struct {
	identity identity.Identity
	err      error
}

func (stub identityVerifierStub) Verify(
	context.Context,
	string,
	time.Time,
) (identity.Identity, error) {
	return stub.identity, stub.err
}

type playbackStub struct {
	asset   media.Asset
	variant media.AudioVariant
	err     error
}

func (stub playbackStub) PlaybackVariant(
	context.Context,
	uuid.UUID,
) (media.Asset, media.AudioVariant, error) {
	return stub.asset, stub.variant, stub.err
}

type objectStorageStub struct {
	object storage.Object
	err    error
	key    string
}

func (stub *objectStorageStub) Open(_ context.Context, key string) (storage.Object, error) {
	stub.key = key
	return stub.object, stub.err
}

func TestPlaybackRequiresVerifiedBearerToken(t *testing.T) {
	dependencies := playbackDependencies()
	dependencies.Tokens = identityVerifierStub{err: identity.ErrInvalidAccessToken}
	request := httptest.NewRequest(http.MethodGet,
		"/v1/media/assets/"+uuid.NewString()+"/playback", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status = %d, authenticate = %q, body = %s",
			response.Code, response.Header().Get("WWW-Authenticate"), response.Body.String())
	}
}

func TestPlaybackStreamsReadyMediaObject(t *testing.T) {
	assetID := uuid.New()
	variantID := uuid.New()
	objectStorage := &objectStorageStub{object: storage.Object{
		Body: io.NopCloser(bytes.NewBufferString("audio bytes")),
		Size: 11, ContentType: "audio/mp4",
	}}
	dependencies := playbackDependencies()
	dependencies.Media = playbackStub{
		asset: media.Asset{ID: assetID, Status: media.StatusReady},
		variant: media.AudioVariant{
			ID: variantID, ObjectKey: "variants/track.m4a", BitrateKbps: 320,
		},
	}
	dependencies.Objects = objectStorage
	request := httptest.NewRequest(http.MethodGet,
		"/v1/media/assets/"+assetID.String()+"/playback", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "audio bytes" {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "audio/mp4" ||
		response.Header().Get("Content-Length") != "11" ||
		response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("headers = %v", response.Header())
	}
	if objectStorage.key != "variants/track.m4a" {
		t.Fatalf("Open() key = %q", objectStorage.key)
	}
}

func TestPlaybackRejectsUnavailableMedia(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "missing", err: media.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "not ready", err: media.ErrNotReady, wantStatus: http.StatusConflict},
		{name: "repository unavailable", err: errors.New("database"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dependencies := playbackDependencies()
			dependencies.Media = playbackStub{err: test.err}
			request := httptest.NewRequest(http.MethodGet,
				"/v1/media/assets/"+uuid.NewString()+"/playback", nil)
			request.Header.Set("Authorization", "Bearer valid")
			response := httptest.NewRecorder()
			NewRouter(dependencies).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s",
					response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestPlaybackCORSPreflightDoesNotRequireToken(t *testing.T) {
	dependencies := playbackDependencies()
	dependencies.Origins = []string{"http://localhost:5173"}
	request := httptest.NewRequest(http.MethodOptions,
		"/v1/media/assets/"+uuid.NewString()+"/playback", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	response := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent ||
		response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("status = %d, headers = %v", response.Code, response.Header())
	}
}

func playbackDependencies() Dependencies {
	return Dependencies{
		Database: databaseStub{}, Storage: storageStub{},
		Tokens: identityVerifierStub{identity: identity.Identity{UserID: uuid.New()}},
		Media:  playbackStub{}, Objects: &objectStorageStub{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}
