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
	"github.com/google/uuid"
)

func TestCreateCreatorRequiresAuthentication(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/creators", strings.NewReader(`{"type":"artist","display_name":"Nova"}`))
	response := httptest.NewRecorder()
	NewRouter(Dependencies{Database: databaseStub{}}).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestCreateCreatorUsesVerifiedIdentity(t *testing.T) {
	userID := uuid.New()
	service := &accountsStub{}
	request := httptest.NewRequest(http.MethodPost, "/v1/creators", strings.NewReader(`{"type":"artist","display_name":"Nova"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, Creators: service}).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.ownerID != userID {
		t.Fatalf("status = %d owner = %s body=%s", response.Code, service.ownerID, response.Body.String())
	}
}

func TestCreatorVerificationQueueUsesModeratorIdentity(t *testing.T) {
	moderatorID := uuid.New()
	pending := creator.VerificationPending
	service := &accountsStub{page: creator.VerificationPage{
		Items: []creator.Account{{
			ID: uuid.New(), Type: creator.TypeArtist, DisplayName: "Nova",
			VerificationStatus: creator.VerificationPending,
		}},
		Limit: 10, Offset: 20, Total: 1,
	}}
	request := httptest.NewRequest(http.MethodGet,
		"/v1/moderation/creators?status=pending&limit=10&offset=20", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{
			UserID: moderatorID, Roles: []string{creator.GlobalRoleModerator},
		}}, Creators: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actor.UserID != moderatorID ||
		service.request.Status == nil || *service.request.Status != pending ||
		service.request.Limit != 10 || service.request.Offset != 20 ||
		!strings.Contains(response.Body.String(), `"total":1`) {
		t.Fatalf("status = %d; stub = %+v; body = %s", response.Code, service, response.Body.String())
	}
}

type tokenStub struct {
	identity identity.Identity
	err      error
}

func (stub tokenStub) Verify(context.Context, string, time.Time) (identity.Identity, error) {
	return stub.identity, stub.err
}

type accountsStub struct {
	ownerID uuid.UUID
	actor   creator.Actor
	request creator.VerificationPageRequest
	page    creator.VerificationPage
}

func (stub *accountsStub) Create(_ context.Context, ownerID uuid.UUID, input creator.CreateInput) (creator.Account, error) {
	stub.ownerID = ownerID
	return creator.Account{ID: uuid.New(), Type: input.Type, DisplayName: input.DisplayName, VerificationStatus: creator.VerificationUnverified}, nil
}
func (*accountsStub) List(context.Context, uuid.UUID) ([]creator.Account, error) { return nil, nil }
func (*accountsStub) ByID(context.Context, uuid.UUID, uuid.UUID) (creator.Account, error) {
	return creator.Account{}, nil
}
func (*accountsStub) Update(context.Context, uuid.UUID, uuid.UUID, creator.UpdateInput) (creator.Account, error) {
	return creator.Account{}, nil
}
func (*accountsStub) SubmitVerification(context.Context, uuid.UUID, uuid.UUID) (creator.Account, error) {
	return creator.Account{}, nil
}
func (*accountsStub) ReviewVerification(context.Context, creator.Actor, uuid.UUID, creator.VerificationStatus) (creator.Account, error) {
	return creator.Account{}, nil
}
func (stub *accountsStub) ListVerification(
	_ context.Context, actor creator.Actor, request creator.VerificationPageRequest,
) (creator.VerificationPage, error) {
	stub.actor, stub.request = actor, request
	return stub.page, nil
}
