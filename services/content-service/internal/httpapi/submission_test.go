package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/submission"
	"github.com/google/uuid"
)

func TestSubmitReleaseUsesAuthenticatedActor(t *testing.T) {
	actorID, releaseID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	service := &submissionStub{result: release.Release{
		ID: releaseID, Status: release.StatusPendingReview,
		SubmittedAt: &now, UpdatedAt: now,
	}}
	request := httptest.NewRequest(http.MethodPost,
		"/v1/releases/"+releaseID.String()+"/submissions", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database:    databaseStub{},
		Tokens:      tokenStub{identity: identity.Identity{UserID: actorID}},
		Submissions: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actorID != actorID ||
		service.releaseID != releaseID ||
		!strings.Contains(response.Body.String(), `"status":"pending_review"`) {
		t.Fatalf("status = %d, actor = %s, release = %s, body = %s",
			response.Code, service.actorID, service.releaseID, response.Body.String())
	}
}

func TestSubmitReleaseMapsReadinessFailure(t *testing.T) {
	releaseID, trackID := uuid.New(), uuid.New()
	service := &submissionStub{err: &submission.ReadinessError{
		TrackID: trackID, Reason: "completed audio upload is required",
	}}
	request := httptest.NewRequest(http.MethodPost,
		"/v1/releases/"+releaseID.String()+"/submissions", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database:    databaseStub{},
		Tokens:      tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Submissions: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusConflict ||
		!strings.Contains(response.Body.String(), "release_not_ready") ||
		!strings.Contains(response.Body.String(), trackID.String()) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestSubmitReleaseRejectsInvalidID(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost,
		"/v1/releases/invalid/submissions", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database:    databaseStub{},
		Tokens:      tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Submissions: &submissionStub{},
	}).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

type submissionStub struct {
	result    release.Release
	err       error
	actorID   uuid.UUID
	releaseID uuid.UUID
}

func (stub *submissionStub) Submit(
	_ context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (release.Release, error) {
	stub.actorID, stub.releaseID = actorID, releaseID
	return stub.result, stub.err
}

var _ ReleaseSubmissions = (*submissionStub)(nil)
