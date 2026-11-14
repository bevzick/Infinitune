package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/submission"
	"github.com/google/uuid"
)

type readinessStub struct {
	result    submission.ReadinessReport
	err       error
	actorID   uuid.UUID
	releaseID uuid.UUID
}

func (stub *readinessStub) Readiness(
	_ context.Context, actorID uuid.UUID, releaseID uuid.UUID,
) (submission.ReadinessReport, error) {
	stub.actorID, stub.releaseID = actorID, releaseID
	return stub.result, stub.err
}

func TestGetReleaseReadinessReturnsTypedIssues(t *testing.T) {
	actorID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New()
	service := &readinessStub{result: submission.ReadinessReport{
		ReleaseID: releaseID, Status: release.StatusDraft,
		Issues: []submission.ReadinessIssue{{
			Code: "media_not_ready", Message: "media processing must be ready", TrackID: &trackID,
		}},
	}}
	request := httptest.NewRequest(http.MethodGet,
		"/v1/releases/"+releaseID.String()+"/readiness", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database:  databaseStub{},
		Tokens:    tokenStub{identity: identity.Identity{UserID: actorID}},
		Readiness: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actorID != actorID || service.releaseID != releaseID {
		t.Fatalf("status=%d actor=%s release=%s body=%s",
			response.Code, service.actorID, service.releaseID, response.Body.String())
	}
	var result submission.ReadinessReport
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil ||
		result.ReadyForSubmission || len(result.Issues) != 1 ||
		result.Issues[0].TrackID == nil || *result.Issues[0].TrackID != trackID {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestGetReleaseReadinessMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		err    error
		status int
	}{
		{name: "invalid uuid", id: "invalid", status: http.StatusUnprocessableEntity},
		{name: "not found", id: uuid.NewString(), err: submission.ErrNotFound, status: http.StatusNotFound},
		{name: "forbidden", id: uuid.NewString(), err: creator.ErrForbidden, status: http.StatusForbidden},
		{name: "media unavailable", id: uuid.NewString(), err: submission.ErrMediaUnavailable, status: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet,
				"/v1/releases/"+test.id+"/readiness", nil)
			request.Header.Set("Authorization", "Bearer access-token")
			response := httptest.NewRecorder()
			NewRouter(Dependencies{
				Database:  databaseStub{},
				Tokens:    tokenStub{identity: identity.Identity{UserID: uuid.New()}},
				Readiness: &readinessStub{err: test.err},
			}).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s",
					response.Code, test.status, response.Body.String())
			}
		})
	}
}

var _ ReleaseReadiness = (*readinessStub)(nil)
