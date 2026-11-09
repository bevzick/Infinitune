package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func TestReviewReleaseUsesAuthenticatedModerator(t *testing.T) {
	releaseID, reviewerID, reviewID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	service := &moderationStub{result: moderation.Review{
		ID: reviewID, ReleaseID: releaseID, ReviewerUserID: reviewerID,
		Decision: moderation.DecisionChangesRequested, Comment: "fix cover",
		ReviewedSubmissionAt: now, ReleaseStatus: release.StatusChangesRequested,
		CreatedAt: now,
	}}
	request := httptest.NewRequest(http.MethodPost,
		"/v1/releases/"+releaseID.String()+"/moderation-reviews",
		strings.NewReader(`{"decision":"changes_requested","comment":"fix cover"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{
			UserID: reviewerID, Roles: []string{creator.GlobalRoleModerator},
		}}, Moderation: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actor.UserID != reviewerID ||
		len(service.actor.Roles) != 1 || service.input.ReleaseID != releaseID ||
		service.input.Decision != moderation.DecisionChangesRequested ||
		!strings.Contains(response.Body.String(), `"release_status":"changes_requested"`) {
		t.Fatalf("status = %d, actor = %+v, input = %+v, body = %s",
			response.Code, service.actor, service.input, response.Body.String())
	}
}

func TestReviewReleaseMapsDomainErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		body string
	}{
		{name: "role", err: moderation.ErrForbidden, code: http.StatusForbidden, body: "moderator_role_required"},
		{name: "self review", err: moderation.ErrSelfReview, code: http.StatusForbidden, body: "self_review_forbidden"},
		{name: "conflict", err: moderation.ErrConflict, code: http.StatusConflict, body: "moderation_review_conflict"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost,
				"/v1/releases/"+uuid.NewString()+"/moderation-reviews",
				strings.NewReader(`{"decision":"approved"}`))
			request.Header.Set("Authorization", "Bearer access-token")
			response := httptest.NewRecorder()
			NewRouter(Dependencies{
				Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}},
				Moderation: &moderationStub{err: test.err},
			}).ServeHTTP(response, request)
			if response.Code != test.code || !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestReviewReleaseRejectsInvalidID(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost,
		"/v1/releases/invalid/moderation-reviews", strings.NewReader(`{"decision":"approved"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Moderation: &moderationStub{},
	}).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

type moderationStub struct {
	result moderation.Review
	err    error
	actor  creator.Actor
	input  moderation.Input
}

func (stub *moderationStub) Review(
	_ context.Context,
	actor creator.Actor,
	input moderation.Input,
) (moderation.Review, error) {
	stub.actor, stub.input = actor, input
	return stub.result, stub.err
}

var _ ReleaseModeration = (*moderationStub)(nil)
