package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/coveringest"
	"github.com/bevzick/Infinitune/services/media-service/internal/identity"
	"github.com/google/uuid"
)

type coverIngestStub struct {
	called      bool
	size        int64
	contentType string
	actorID     uuid.UUID
	result      coveringest.Result
	created     bool
	err         error
}

func (s *coverIngestStub) Ingest(_ context.Context, actor, upload uuid.UUID, contentType string, size int64, body io.Reader) (coveringest.Result, bool, error) {
	s.called = true
	s.actorID = actor
	s.size = size
	s.contentType = contentType
	_, _ = io.ReadAll(body)
	return s.result, s.created, s.err
}
func TestCoverHTTPAuthenticatedRawBody(t *testing.T) {
	actor, upload, release := uuid.New(), uuid.New(), uuid.New()
	stub := &coverIngestStub{created: true, result: coveringest.Result{ReleaseID: release, Status: content.StatusCompleted}}
	router := NewRouter(Dependencies{Covers: stub, Tokens: identityVerifierStub{identity: identity.Identity{UserID: actor}}})
	request := httptest.NewRequest(http.MethodPut, "/v1/media/cover-uploads/"+upload.String(), bytes.NewReader([]byte("cover")))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "image/png")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !stub.called || stub.size != 5 || stub.contentType != "image/png" || stub.actorID != actor || !strings.Contains(response.Body.String(), release.String()) {
		t.Fatalf("status=%d body=%s stub=%+v", response.Code, response.Body.String(), stub)
	}
}
func TestCoverHTTPRejectsInvalidAndLargeRequests(t *testing.T) {
	upload := uuid.New()
	stub := &coverIngestStub{}
	router := NewRouter(Dependencies{Covers: stub, Tokens: identityVerifierStub{identity: identity.Identity{UserID: uuid.New()}}})
	tests := []struct {
		name                string
		path, authorization string
		length              int64
		status              int
	}{
		{"unauthorized", "/v1/media/cover-uploads/" + upload.String(), "", 5, http.StatusUnauthorized},
		{"invalid id", "/v1/media/cover-uploads/bad", "Bearer token", 5, http.StatusUnprocessableEntity},
		{"too large", "/v1/media/cover-uploads/" + upload.String(), "Bearer token", coveringest.MaximumCoverBytes + 1, http.StatusRequestEntityTooLarge},
		{"length required", "/v1/media/cover-uploads/" + upload.String(), "Bearer token", -1, http.StatusLengthRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, tt.path, bytes.NewReader([]byte("cover")))
			request.ContentLength = tt.length
			if tt.authorization != "" {
				request.Header.Set("Authorization", tt.authorization)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, tt.status, response.Body.String())
			}
		})
	}
	if stub.called {
		t.Fatal("ingest called on invalid request")
	}
}
func TestCoverHTTPMapsDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{content.ErrForbidden, 403}, {content.ErrNotFound, 404}, {content.ErrConflict, 409},
		{content.ErrInvalidTransition, 409}, {coveringest.ErrInvalid, 422}, {coveringest.ErrTooLarge, 413},
		{coveringest.ErrStorage, 503}, {errors.New("unexpected"), 500},
	} {
		stub := &coverIngestStub{err: tc.err}
		router := NewRouter(Dependencies{Covers: stub, Tokens: identityVerifierStub{identity: identity.Identity{UserID: uuid.New()}}})
		request := httptest.NewRequest(http.MethodPut, "/v1/media/cover-uploads/"+uuid.New().String(), strings.NewReader("x"))
		request.Header.Set("Authorization", "Bearer token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("error %v => %d, want %d", tc.err, response.Code, tc.status)
		}
	}
}
