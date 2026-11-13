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

func TestListReleaseModerationReviewsReturnsAuditPage(t *testing.T) {
	now := time.Now().UTC()
	service := &moderationStub{page: moderation.Page{
		Items: []moderation.Review{{
			ID: uuid.New(), ReleaseID: uuid.New(), ReviewerUserID: uuid.New(),
			Decision: moderation.DecisionRejected, Comment: "copyright mismatch",
			ReviewedSubmissionAt: now, ReleaseStatus: release.StatusRejected, CreatedAt: now,
		}}, Limit: 10, Offset: 20, Total: 31,
	}}
	request := httptest.NewRequest(http.MethodGet,
		"/v1/moderation/release-reviews?limit=10&offset=20", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{
			UserID: uuid.New(), Roles: []string{creator.GlobalRoleAdmin},
		}}, Moderation: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.pageRequest.Limit != 10 ||
		service.pageRequest.Offset != 20 || !strings.Contains(response.Body.String(), `"total":31`) ||
		!strings.Contains(response.Body.String(), `"comment":"copyright mismatch"`) {
		t.Fatalf("status = %d, request = %+v, body = %s", response.Code, service.pageRequest, response.Body.String())
	}
}

type moderationStub struct {
	result moderation.Review
	err    error
	actor  creator.Actor
	input  moderation.Input
	page        moderation.Page
	pageRequest moderation.PageRequest
}

func (stub *moderationStub) List(
	_ context.Context,
	actor creator.Actor,
	page moderation.PageRequest,
) (moderation.Page, error) {
	stub.actor, stub.pageRequest = actor, page
	return stub.page, stub.err
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
