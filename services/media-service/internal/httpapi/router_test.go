package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/identity"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/source"
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
	quality *media.AudioQuality
}

type ingestStub struct {
	asset       media.Asset
	created     bool
	err         error
	actorID     uuid.UUID
	uploadID    uuid.UUID
	contentType string
	sizeBytes   int64
	body        string
}

type externalImportStub struct {
	asset          media.Asset
	created        bool
	err            error
	actorID        uuid.UUID
	contentTrackID uuid.UUID
	provider       source.ProviderName
	externalID     string
}

func (stub *externalImportStub) Import(
	_ context.Context,
	actorID uuid.UUID,
	contentTrackID uuid.UUID,
	provider source.ProviderName,
	externalID string,
) (media.Asset, bool, error) {
	stub.actorID = actorID
	stub.contentTrackID = contentTrackID
	stub.provider = provider
	stub.externalID = externalID
	return stub.asset, stub.created, stub.err
}

func (stub *ingestStub) Ingest(
	_ context.Context,
	actorID uuid.UUID,
	uploadID uuid.UUID,
	contentType string,
	sizeBytes int64,
	body io.Reader,
) (media.Asset, bool, error) {
	stub.actorID = actorID
	stub.uploadID = uploadID
	stub.contentType = contentType
	stub.sizeBytes = sizeBytes
	data, _ := io.ReadAll(body)
	stub.body = string(data)
	return stub.asset, stub.created, stub.err
}

func (stub playbackStub) PlaybackVariant(
	_ context.Context,
	_ uuid.UUID,
	quality media.AudioQuality,
) (media.Asset, media.AudioVariant, error) {
	if stub.quality != nil {
		*stub.quality = quality
	}
	return stub.asset, stub.variant, stub.err
}

type objectStorageStub struct {
	object storage.Object
	err    error
	key    string
}

type readSeekCloser struct {
	*bytes.Reader
}

func (readSeekCloser) Close() error { return nil }

func newReadSeekCloser(value string) readSeekCloser {
	return readSeekCloser{Reader: bytes.NewReader([]byte(value))}
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

func TestCreatorUploadIngestsAuthenticatedRawAudio(t *testing.T) {
	actorID, uploadID, assetID, contentTrackID :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ingester := &ingestStub{created: true, asset: media.Asset{
		ID: assetID, ContentTrackID: contentTrackID, Status: media.StatusUploaded,
	}}
	dependencies := playbackDependencies()
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: actorID}}
	dependencies.Ingest = ingester
	request := httptest.NewRequest(http.MethodPut,
		"/v1/media/uploads/"+uploadID.String(), bytes.NewBufferString("audio"))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "audio/flac")
	response := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || ingester.actorID != actorID ||
		ingester.uploadID != uploadID || ingester.contentType != "audio/flac" ||
		ingester.sizeBytes != 5 || ingester.body != "audio" {
		t.Fatalf("status = %d, ingest = %+v, body = %s",
			response.Code, ingester, response.Body.String())
	}
}

func TestCreatorUploadRequiresContentLength(t *testing.T) {
	dependencies := playbackDependencies()
	dependencies.Ingest = &ingestStub{}
	request := httptest.NewRequest(http.MethodPut,
		"/v1/media/uploads/"+uuid.NewString(), bytes.NewBufferString("audio"))
	request.ContentLength = -1
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "audio/flac")
	response := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(response, request)
	if response.Code != http.StatusLengthRequired {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAuthenticatedUserImportsExternalTrack(t *testing.T) {
	actorID, trackID, assetID := uuid.New(), uuid.New(), uuid.New()
	service := &externalImportStub{created: true, asset: media.Asset{
		ID: assetID, ContentTrackID: trackID, Status: media.StatusUploaded,
	}}
	dependencies := playbackDependencies()
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: actorID}}
	dependencies.Imports = service
	request := httptest.NewRequest(http.MethodPost, "/v1/media/imports", strings.NewReader(`{
		"content_track_id":"`+trackID.String()+`",
		"provider":"jamendo",
		"external_id":"42"
	}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.actorID != actorID ||
		service.contentTrackID != trackID || service.provider != source.ProviderJamendo ||
		service.externalID != "42" {
		t.Fatalf("status = %d, import = %+v, body = %s", response.Code, service, response.Body.String())
	}
}

func TestExternalImportRejectsUnknownProvider(t *testing.T) {
	dependencies := playbackDependencies()
	dependencies.Imports = &externalImportStub{}
	request := httptest.NewRequest(http.MethodPost, "/v1/media/imports", strings.NewReader(`{
		"content_track_id":"`+uuid.NewString()+`",
		"provider":"unknown",
		"external_id":"42"
	}`))
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestPlaybackStreamsReadyMediaObject(t *testing.T) {
	assetID := uuid.New()
	variantID := uuid.New()
	var requestedQuality media.AudioQuality
	objectStorage := &objectStorageStub{object: storage.Object{
		Body: newReadSeekCloser("audio bytes"),
		Size: 11, ContentType: "audio/mp4",
	}}
	dependencies := playbackDependencies()
	dependencies.Media = playbackStub{
		asset: media.Asset{ID: assetID, Status: media.StatusReady},
		variant: media.AudioVariant{
			ID: variantID, ObjectKey: "variants/track.m4a", Codec: "mp3", BitrateKbps: 128,
		},
		quality: &requestedQuality,
	}
	dependencies.Objects = objectStorage
	request := httptest.NewRequest(http.MethodGet,
		"/v1/media/assets/"+assetID.String()+"/playback?quality=low", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "audio bytes" {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "audio/mp4" ||
		response.Header().Get("Content-Length") != "11" ||
		response.Header().Get("Accept-Ranges") != "bytes" ||
		response.Header().Get("Cache-Control") != "private, no-store" ||
		response.Header().Get("X-Audio-Quality") != "low" ||
		response.Header().Get("X-Audio-Codec") != "mp3" ||
		response.Header().Get("X-Audio-Bitrate-Kbps") != "128" ||
		requestedQuality != media.AudioQualityLow {
		t.Fatalf("headers = %v", response.Header())
	}
	if objectStorage.key != "variants/track.m4a" {
		t.Fatalf("Open() key = %q", objectStorage.key)
	}
}

func TestPlaybackRejectsUnknownAudioQuality(t *testing.T) {
	var requestedQuality media.AudioQuality
	dependencies := playbackDependencies()
	dependencies.Media = playbackStub{quality: &requestedQuality}
	request := httptest.NewRequest(http.MethodGet,
		"/v1/media/assets/"+uuid.NewString()+"/playback?quality=studio", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || requestedQuality != "" ||
		!bytes.Contains(response.Body.Bytes(), []byte(`"code":"invalid_audio_quality"`)) {
		t.Fatalf("status = %d, quality = %q, body = %s",
			response.Code, requestedQuality, response.Body.String())
	}
}

func TestPlaybackStreamsRequestedByteRange(t *testing.T) {
	assetID := uuid.New()
	dependencies := playbackDependencies()
	dependencies.Media = playbackStub{
		asset:   media.Asset{ID: assetID, Status: media.StatusReady},
		variant: media.AudioVariant{ID: uuid.New(), ObjectKey: "variants/track.m4a"},
	}
	dependencies.Objects = &objectStorageStub{object: storage.Object{
		Body: newReadSeekCloser("0123456789"), Size: 10, ContentType: "audio/mp4",
	}}
	request := httptest.NewRequest(http.MethodGet,
		"/v1/media/assets/"+assetID.String()+"/playback", nil)
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent || response.Body.String() != "2345" {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Range") != "bytes 2-5/10" ||
		response.Header().Get("Content-Length") != "4" ||
		response.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("headers = %v", response.Header())
	}
}

func TestPlaybackRejectsUnsatisfiableByteRange(t *testing.T) {
	assetID := uuid.New()
	dependencies := playbackDependencies()
	dependencies.Media = playbackStub{
		asset:   media.Asset{ID: assetID, Status: media.StatusReady},
		variant: media.AudioVariant{ID: uuid.New(), ObjectKey: "variants/track.m4a"},
	}
	dependencies.Objects = &objectStorageStub{object: storage.Object{
		Body: newReadSeekCloser("0123456789"), Size: 10, ContentType: "audio/mp4",
	}}
	request := httptest.NewRequest(http.MethodGet,
		"/v1/media/assets/"+assetID.String()+"/playback", nil)
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Range", "bytes=20-30")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusRequestedRangeNotSatisfiable ||
		response.Header().Get("Content-Range") != "bytes */10" ||
		response.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("status = %d, headers = %v, body = %s",
			response.Code, response.Header(), response.Body.String())
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
		{name: "not published", err: media.ErrNotPlayable, wantStatus: http.StatusNotFound},
		{name: "access unavailable", err: media.ErrAccessUnavailable, wantStatus: http.StatusServiceUnavailable},
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
		response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		response.Header().Get("Access-Control-Expose-Headers") == "" {
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
