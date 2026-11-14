package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/bevzick/Infinitune/services/comment-service/internal/identity"
	"github.com/bevzick/Infinitune/services/comment-service/internal/moderation"
	"github.com/google/uuid"
)

func TestApplyModerationActionUsesVerifiedModerator(t *testing.T) {
	moderatorID, reportID := uuid.New(), uuid.New()
	service := &moderationStub{}
	router := NewRouter(Dependencies{
		Database:   databaseStub{},
		Tokens:     tokenStub{identity: identity.Identity{UserID: moderatorID, Roles: []string{"moderator"}}},
		Moderation: service,
	})
	request := httptest.NewRequest(http.MethodPost,
		"/v1/moderation/comment-reports/"+reportID.String()+"/actions",
		strings.NewReader(`{"action":"hide_comment","note":"Policy violation"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actor.UserID != moderatorID ||
		service.input.ReportID != reportID || service.input.Action != moderation.ActionHideComment {
		t.Fatalf("status = %d, actor/input = %+v/%+v; body = %s",
			response.Code, service.actor, service.input, response.Body.String())
	}
}

func TestListModerationCasesForwardsPrivateQueueOnlyToService(t *testing.T) {
	status := comment.ReportStatusPending
	service := &moderationStub{pageResult: moderation.Page{
		Items: []moderation.Case{{
			Report:  comment.Report{ID: uuid.New(), ReporterUserID: uuid.New(), Status: status, CreatedAt: time.Now()},
			Comment: comment.Comment{ID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(), Content: "Reported", Status: comment.StatusPublished},
		}}, Limit: 5, Offset: 2, Total: 1,
	}}
	router := NewRouter(Dependencies{
		Database:   databaseStub{},
		Tokens:     tokenStub{identity: identity.Identity{UserID: uuid.New(), Roles: []string{"admin"}}},
		Moderation: service,
	})
	request := httptest.NewRequest(http.MethodGet,
		"/v1/moderation/comment-reports?status=pending&limit=5&offset=2", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.page.Status == nil || *service.page.Status != status ||
		service.page.Limit != 5 || service.page.Offset != 2 {
		t.Fatalf("status/page = %d/%+v; body = %s", response.Code, service.page, response.Body.String())
	}
}

func TestModerationErrorsAreMapped(t *testing.T) {
	service := &moderationStub{err: moderation.ErrForbidden}
	router := NewRouter(Dependencies{
		Database:   databaseStub{},
		Tokens:     tokenStub{identity: identity.Identity{UserID: uuid.New(), Roles: []string{"listener"}}},
		Moderation: service,
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/moderation/comment-reports", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "moderator_role_required") {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
}

func TestListModerationActionsReturnsAuditPage(t *testing.T) {
	service := &moderationStub{actionResult: moderation.ActionPage{
		Items: []moderation.Decision{{
			ID: uuid.New(), ReportID: uuid.New(), ModeratorUserID: uuid.New(),
			Action: moderation.ActionHideComment, Note: "Policy violation", CreatedAt: time.Now(),
		}}, Limit: 10, Offset: 20, Total: 21,
	}}
	router := NewRouter(Dependencies{
		Database: databaseStub{},
		Tokens: tokenStub{identity: identity.Identity{
			UserID: uuid.New(), Roles: []string{"admin"},
		}},
		Moderation: service,
	})
	request := httptest.NewRequest(http.MethodGet,
		"/v1/moderation/comment-actions?limit=10&offset=20", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actionPage.Limit != 10 ||
		service.actionPage.Offset != 20 || !strings.Contains(response.Body.String(), `"total":21`) ||
		!strings.Contains(response.Body.String(), `"note":"Policy violation"`) {
		t.Fatalf("status = %d; request = %+v; body = %s", response.Code, service.actionPage, response.Body.String())
	}
}

type moderationStub struct {
	actor      moderation.Actor
	input      moderation.Input
	page       moderation.PageRequest
	pageResult moderation.Page
	err        error
	actionPage   moderation.ActionPageRequest
	actionResult moderation.ActionPage
}

func (stub *moderationStub) ListActions(
	_ context.Context,
	actor moderation.Actor,
	page moderation.ActionPageRequest,
) (moderation.ActionPage, error) {
	stub.actor, stub.actionPage = actor, page
	return stub.actionResult, stub.err
}

func (stub *moderationStub) List(_ context.Context, actor moderation.Actor, page moderation.PageRequest) (moderation.Page, error) {
	stub.actor, stub.page = actor, page
	return stub.pageResult, stub.err
}

func (stub *moderationStub) Apply(_ context.Context, actor moderation.Actor, input moderation.Input) (moderation.Decision, error) {
	stub.actor, stub.input = actor, input
	return moderation.Decision{
		ID: uuid.New(), ReportID: input.ReportID, ModeratorUserID: actor.UserID,
		Action: input.Action, Note: input.Note,
		PreviousReportStatus: comment.ReportStatusPending, ResultingReportStatus: comment.ReportStatusReviewing,
		PreviousCommentStatus: comment.StatusPublished, ResultingCommentStatus: comment.StatusHidden,
		CreatedAt: time.Now(),
	}, stub.err
}
