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
	"github.com/google/uuid"
)

func TestCreateReleaseUsesAuthenticatedCreatorMember(t *testing.T) {
	actorID, creatorID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	service := &releasesStub{draft: release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Single",
			Type: release.TypeSingle, ReleaseDate: time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC),
			Status: release.StatusDraft,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Single", DiscNumber: 1, TrackNumber: 1,
		}},
	}}
	request := httptest.NewRequest(http.MethodPost, "/v1/releases", strings.NewReader(`{
		"creator_id":"`+creatorID.String()+`","title":"Single","release_type":"single",
		"release_date":"2027-01-02","tracks":[{"title":"Single","track_number":1,"disc_number":1}]
	}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: actorID}},
		Releases: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.actorID != actorID ||
		service.createInput.CreatorID != creatorID || len(service.createInput.Tracks) != 1 ||
		!strings.Contains(response.Body.String(), `"status":"draft"`) {
		t.Fatalf("status = %d; stub = %+v; body = %s", response.Code, service, response.Body.String())
	}
}

func TestArchiveReleaseMapsCatalogFailure(t *testing.T) {
	service := &releasesStub{err: release.ErrCatalogUnavailable}
	request := httptest.NewRequest(http.MethodDelete,
		"/v1/releases/"+uuid.NewString(), nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}},
		Releases: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable ||
		!strings.Contains(response.Body.String(), "catalog_unavailable") {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
}

type releasesStub struct {
	draft       release.Draft
	result      release.Release
	err         error
	actorID     uuid.UUID
	releaseID   uuid.UUID
	createInput release.CreateDraftInput
	updateInput release.UpdateInput
}

func (stub *releasesStub) CreateDraft(
	_ context.Context,
	actorID uuid.UUID,
	input release.CreateDraftInput,
) (release.Draft, error) {
	stub.actorID, stub.createInput = actorID, input
	return stub.draft, stub.err
}

func (stub *releasesStub) ByID(
	_ context.Context, actorID uuid.UUID, releaseID uuid.UUID,
) (release.Draft, error) {
	stub.actorID, stub.releaseID = actorID, releaseID
	return stub.draft, stub.err
}

func (stub *releasesStub) Update(
	_ context.Context, actorID uuid.UUID, releaseID uuid.UUID, input release.UpdateInput,
) (release.Draft, error) {
	stub.actorID, stub.releaseID, stub.updateInput = actorID, releaseID, input
	return stub.draft, stub.err
}

func (stub *releasesStub) Withdraw(
	_ context.Context, actorID uuid.UUID, releaseID uuid.UUID,
) (release.Release, error) {
	stub.actorID, stub.releaseID = actorID, releaseID
	return stub.result, stub.err
}

func (stub *releasesStub) Archive(
	_ context.Context, actorID uuid.UUID, releaseID uuid.UUID,
) (release.Release, error) {
	stub.actorID, stub.releaseID = actorID, releaseID
	return stub.result, stub.err
}

var _ Releases = (*releasesStub)(nil)
