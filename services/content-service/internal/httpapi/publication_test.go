package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/bevzick/Infinitune/services/content-service/internal/publication"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func TestPublishReleaseUsesAuthenticatedActor(t *testing.T) {
	actorID, releaseID, albumID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	service := &publicationStub{result: release.Release{
		ID: releaseID, Status: release.StatusPublished,
		CatalogAlbumID: &albumID, PublishedAt: &now, UpdatedAt: now,
	}}
	request := httptest.NewRequest(http.MethodPost,
		"/v1/releases/"+releaseID.String()+"/publications", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database:     databaseStub{},
		Tokens:       tokenStub{identity: identity.Identity{UserID: actorID}},
		Publications: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actorID != actorID ||
		service.releaseID != releaseID ||
		!strings.Contains(response.Body.String(), `"status":"published"`) ||
		!strings.Contains(response.Body.String(), albumID.String()) {
		t.Fatalf("status = %d, actor = %s, release = %s, body = %s",
			response.Code, service.actorID, service.releaseID, response.Body.String())
	}
}

func TestPublishReleaseMapsCatalogUnavailable(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost,
		"/v1/releases/"+uuid.NewString()+"/publications", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database:     databaseStub{},
		Tokens:       tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Publications: &publicationStub{err: publication.ErrCatalogUnavailable},
	}).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable ||
		!strings.Contains(response.Body.String(), "catalog_unavailable") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestPublishReleaseRejectsInvalidID(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/releases/invalid/publications", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database:     databaseStub{},
		Tokens:       tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Publications: &publicationStub{},
	}).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

type publicationStub struct {
	result    release.Release
	err       error
	actorID   uuid.UUID
	releaseID uuid.UUID
}

func (stub *publicationStub) Publish(
	_ context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (release.Release, error) {
	stub.actorID, stub.releaseID = actorID, releaseID
	return stub.result, stub.err
}

var _ ReleasePublications = (*publicationStub)(nil)
