package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/coverupload"
	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/google/uuid"
)

func TestCreateCoverUploadSessionAPI(t *testing.T) {
	releaseID, actorID, sessionID, key := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	service := &coverSessionsStub{result: coverupload.Session{
		ID: sessionID, ReleaseID: releaseID, Status: coverupload.StatusPending,
		OriginalName: "cover.webp", MIMEType: "image/webp", SizeBytes: 2048,
		CreatedAt: now, UpdatedAt: now,
	}}
	request := httptest.NewRequest(http.MethodPost, "/v1/releases/"+releaseID.String()+"/cover-uploads", strings.NewReader(`{
       "idempotency_key":"`+key.String()+`", "original_name":"cover.webp",
       "mime_type":"image/webp", "size_bytes":2048
    }`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: actorID}},
		CoverUploads: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.actorID != actorID ||
		service.input.ReleaseID != releaseID || service.input.IdempotencyKey != key ||
		response.Header().Get("Location") != "/v1/media/cover-uploads/"+sessionID.String() ||
		!strings.Contains(response.Body.String(), `"upload_url":"/v1/media/cover-uploads/`+sessionID.String()+`"`) ||
		strings.Contains(response.Body.String(), "object_key") {
		t.Fatalf("status = %d, location = %q, service = %+v, body = %s", response.Code, response.Header().Get("Location"), service, response.Body.String())
	}
}

func TestCreateCoverUploadSessionAPIErrors(t *testing.T) {
	cases := []struct {
		name, releaseID, body string
		serviceErr            error
		expectedStatus        int
		expectedCode          string
	}{
		{"invalid release", "invalid", `{}`, nil, 422, "invalid_cover_upload"},
		{"nil UUID", uuid.Nil.String(), `{}`, nil, 422, "invalid_cover_upload"},
		{"invalid key", uuid.NewString(), `{"idempotency_key":"invalid"}`, nil, 422, "invalid_cover_upload"},
		{"invalid MIME", uuid.NewString(), `{"idempotency_key":"` + uuid.NewString() + `","original_name":"x.svg","mime_type":"image/svg+xml","size_bytes":1}`, coverupload.ErrInvalid, 422, "invalid_cover_upload"},
		{"missing release", uuid.NewString(), `{"idempotency_key":"` + uuid.NewString() + `"}`, coverupload.ErrNotFound, 404, "release_not_found"},
		{"replay collision", uuid.NewString(), `{"idempotency_key":"` + uuid.NewString() + `"}`, coverupload.ErrIdempotencyConflict, 409, "cover_upload_idempotency_conflict"},
		{"locked release", uuid.NewString(), `{"idempotency_key":"` + uuid.NewString() + `"}`, coverupload.ErrReleaseLocked, 409, "release_cover_locked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/releases/"+tc.releaseID+"/cover-uploads", strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer access-token")
			response := httptest.NewRecorder()
			NewRouter(Dependencies{
				Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}},
				CoverUploads: &coverSessionsStub{err: tc.serviceErr},
			}).ServeHTTP(response, request)
			if response.Code != tc.expectedStatus || !strings.Contains(response.Body.String(), tc.expectedCode) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

type coverSessionsStub struct {
	result  coverupload.Session
	err     error
	actorID uuid.UUID
	input   coverupload.CreateInput
}

func (stub *coverSessionsStub) CreateSession(_ context.Context, actorID uuid.UUID, input coverupload.CreateInput) (coverupload.Session, error) {
	stub.actorID, stub.input = actorID, input
	return stub.result, stub.err
}

var _ CoverUploadSessions = (*coverSessionsStub)(nil)
