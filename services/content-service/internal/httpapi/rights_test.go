package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/google/uuid"
)

func TestSetTrackRightsUsesAuthenticatedActor(t *testing.T) {
	actorID, trackID, rightID := uuid.New(), uuid.New(), uuid.New()
	service := &trackRightsStub{stored: rights.Right{
		ID: rightID, ReleaseTrackID: trackID, TerritoryCode: "UA",
		Type:            rights.TypeStreaming,
		ValidFrom:       time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		GrantedByUserID: actorID,
	}}
	request := httptest.NewRequest(http.MethodPut,
		"/v1/release-tracks/"+trackID.String()+"/rights",
		strings.NewReader(`{"territory_code":"UA","rights_type":"streaming","valid_from":"2027-01-01","valid_until":null}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{},
		Tokens:   tokenStub{identity: identity.Identity{UserID: actorID}}, Rights: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.actorID != actorID ||
		service.input.ReleaseTrackID != trackID || service.input.ValidUntil != nil {
		t.Fatalf("status = %d, actor = %s, input = %+v, body = %s",
			response.Code, service.actorID, service.input, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), rightID.String()) ||
		!strings.Contains(response.Body.String(), `"valid_from":"2027-01-01"`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestSetTrackRightsRejectsInvalidDateBeforeService(t *testing.T) {
	actorID, trackID := uuid.New(), uuid.New()
	service := &trackRightsStub{}
	request := httptest.NewRequest(http.MethodPut,
		"/v1/release-tracks/"+trackID.String()+"/rights",
		strings.NewReader(`{"territory_code":"UA","rights_type":"streaming","valid_from":"01-01-2027"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{},
		Tokens:   tokenStub{identity: identity.Identity{UserID: actorID}}, Rights: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || service.setCalls != 0 {
		t.Fatalf("status = %d, calls = %d, body = %s",
			response.Code, service.setCalls, response.Body.String())
	}
}

func TestListAndDeleteTrackRights(t *testing.T) {
	actorID, trackID, rightID := uuid.New(), uuid.New(), uuid.New()
	service := &trackRightsStub{listed: []rights.Right{{
		ID: rightID, ReleaseTrackID: trackID, TerritoryCode: rights.TerritoryWorld,
		Type: rights.TypeDistribution, ValidFrom: time.Now(),
		GrantedByUserID: actorID,
	}}}
	router := NewRouter(Dependencies{
		Database: databaseStub{},
		Tokens:   tokenStub{identity: identity.Identity{UserID: actorID}}, Rights: service,
	})
	listRequest := httptest.NewRequest(http.MethodGet,
		"/v1/release-tracks/"+trackID.String()+"/rights", nil)
	listRequest.Header.Set("Authorization", "Bearer access-token")
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || service.listTrackID != trackID ||
		!strings.Contains(listResponse.Body.String(), rightID.String()) {
		t.Fatalf("list status = %d, track = %s, body = %s",
			listResponse.Code, service.listTrackID, listResponse.Body.String())
	}
	deleteRequest := httptest.NewRequest(http.MethodDelete,
		"/v1/track-rights/"+rightID.String(), nil)
	deleteRequest.Header.Set("Authorization", "Bearer access-token")
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || service.deletedID != rightID {
		t.Fatalf("delete status = %d, right = %s, body = %s",
			deleteResponse.Code, service.deletedID, deleteResponse.Body.String())
	}
}

func TestTrackRightsImmutableMapsToConflict(t *testing.T) {
	actorID, trackID := uuid.New(), uuid.New()
	service := &trackRightsStub{err: rights.ErrImmutable}
	request := httptest.NewRequest(http.MethodPut,
		"/v1/release-tracks/"+trackID.String()+"/rights",
		strings.NewReader(`{"territory_code":"US","rights_type":"download","valid_from":"2027-01-01"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{},
		Tokens:   tokenStub{identity: identity.Identity{UserID: actorID}}, Rights: service,
	}).ServeHTTP(response, request)
	if response.Code != http.StatusConflict ||
		!strings.Contains(response.Body.String(), "track_rights_immutable") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestTrackRightsPreflightAllowsPut(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions,
		"/v1/release-tracks/"+uuid.NewString()+"/rights", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{
		Database: databaseStub{}, Origins: []string{"http://localhost:5173"},
	}).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent ||
		!strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), "PUT") {
		t.Fatalf("status = %d, methods = %q",
			response.Code, response.Header().Get("Access-Control-Allow-Methods"))
	}
}

type trackRightsStub struct {
	stored      rights.Right
	listed      []rights.Right
	err         error
	actorID     uuid.UUID
	input       rights.SetInput
	listTrackID uuid.UUID
	deletedID   uuid.UUID
	setCalls    int
}

func (stub *trackRightsStub) Set(
	_ context.Context,
	actorID uuid.UUID,
	input rights.SetInput,
) (rights.Right, error) {
	stub.setCalls++
	stub.actorID, stub.input = actorID, input
	return stub.stored, stub.err
}

func (stub *trackRightsStub) List(
	_ context.Context,
	actorID uuid.UUID,
	trackID uuid.UUID,
) ([]rights.Right, error) {
	stub.actorID, stub.listTrackID = actorID, trackID
	return stub.listed, stub.err
}

func (stub *trackRightsStub) Delete(
	_ context.Context,
	actorID uuid.UUID,
	rightID uuid.UUID,
) error {
	stub.actorID, stub.deletedID = actorID, rightID
	return stub.err
}

var _ TrackRights = (*trackRightsStub)(nil)
