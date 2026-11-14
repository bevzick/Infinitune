package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/google/uuid"
)

func TestCreateUploadSessionUsesAuthenticatedActorAndTrack(t *testing.T) {
	actorID, trackID, uploadID, idempotencyKey := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	service := &uploadSessionsStub{result: upload.Session{
		ID: uploadID, ReleaseTrackID: trackID, OriginalName: "single.wav",
		MIMEType: "audio/wav", SizeBytes: 1024, Status: upload.StatusPending,
		CreatedAt: now, UpdatedAt: now,
	}}
	request := httptest.NewRequest(http.MethodPost,
		"/v1/release-tracks/"+trackID.String()+"/uploads", strings.NewReader(`{
			"idempotency_key":"`+idempotencyKey.String()+`",
			"original_name":"single.wav","mime_type":"audio/wav","size_bytes":1024
		}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: actorID}},
		Uploads: service,
	}).ServeHTTP(response, request)

	if response.Code != http.StatusCreated || service.actorID != actorID ||
		service.input.ReleaseTrackID != trackID || service.input.IdempotencyKey != idempotencyKey ||
		response.Header().Get("Location") != "/v1/media/uploads/"+uploadID.String() ||
		!strings.Contains(response.Body.String(), `"upload_url":"/v1/media/uploads/`+uploadID.String()+`"`) {
		t.Fatalf("status = %d, location = %q, stub = %+v, body = %s",
			response.Code, response.Header().Get("Location"), service, response.Body.String())
	}
}

func TestCreateUploadSessionRejectsInvalidIdempotencyKey(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost,
		"/v1/release-tracks/"+uuid.NewString()+"/uploads", strings.NewReader(`{
			"idempotency_key":"invalid","original_name":"single.wav",
			"mime_type":"audio/wav","size_bytes":1024
		}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Uploads: &uploadSessionsStub{},
	}).ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(response.Body.String(), "invalid_upload_session") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCreateUploadSessionMapsIdempotencyConflict(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost,
		"/v1/release-tracks/"+uuid.NewString()+"/uploads", strings.NewReader(`{
			"idempotency_key":"`+uuid.NewString()+`","original_name":"single.wav",
			"mime_type":"audio/wav","size_bytes":1024
		}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Uploads: &uploadSessionsStub{err: upload.ErrIdempotencyConflict},
	}).ServeHTTP(response, request)

	if response.Code != http.StatusConflict ||
		!strings.Contains(response.Body.String(), "upload_idempotency_conflict") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

type uploadSessionsStub struct {
	result  upload.Session
	err     error
	actorID uuid.UUID
	input   upload.CreateInput
}

func (stub *uploadSessionsStub) CreateSession(
	_ context.Context, actorID uuid.UUID, input upload.CreateInput,
) (upload.Session, error) {
	stub.actorID, stub.input = actorID, input
	return stub.result, stub.err
}

var _ UploadSessions = (*uploadSessionsStub)(nil)
