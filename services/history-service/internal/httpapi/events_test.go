package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/bevzick/Infinitune/services/history-service/internal/identity"
	"github.com/google/uuid"
)

func TestRecordEventUsesVerifiedIdentityAndRejectsSpoofing(t *testing.T) {
	userID, eventID, trackID := uuid.New(), uuid.New(), uuid.New()
	service := &historyStub{result: history.RecordResult{EventID: eventID, Recorded: true, Completed: true}}
	router := NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, History: service})
	body := `{"event_id":"` + eventID.String() + `","track_id":"` + trackID.String() + `","started_at":"2026-11-10T11:58:00Z","finished_at":"2026-11-10T12:00:00Z","listened_ms":90000,"context_type":"direct","device_type":"web"}`
	request := httptest.NewRequest(http.MethodPost, "/v1/history/events", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.userID != userID || service.input.TrackID != trackID || !strings.Contains(response.Body.String(), `"recorded":true`) {
		t.Fatalf("status=%d user=%s input=%+v body=%s", response.Code, service.userID, service.input, response.Body.String())
	}

	spoofed := strings.Replace(body, `"track_id"`, `"user_id":"`+uuid.NewString()+`","track_id"`, 1)
	request = httptest.NewRequest(http.MethodPost, "/v1/history/events", strings.NewReader(spoofed))
	request.Header.Set("Authorization", "Bearer token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.calls != 1 {
		t.Fatalf("spoof status=%d calls=%d body=%s", response.Code, service.calls, response.Body.String())
	}
}

func TestRecordEventRequiresAuthenticationAndMapsDomainErrors(t *testing.T) {
	service := &historyStub{}
	router := NewRouter(Dependencies{Database: databaseStub{}, History: service})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/history/events", strings.NewReader(`{}`)))
	if response.Code != http.StatusUnauthorized || service.calls != 0 {
		t.Fatalf("status=%d calls=%d", response.Code, service.calls)
	}

	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{"invalid", history.ErrInvalidArgument, http.StatusUnprocessableEntity},
		{"missing track", history.ErrTrackNotFound, http.StatusNotFound},
		{"conflict", history.ErrEventConflict, http.StatusConflict},
		{"catalog down", history.ErrCatalogUnavailable, http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			failing := &historyStub{err: test.err}
			authenticated := NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}}, History: failing})
			request := httptest.NewRequest(http.MethodPost, "/v1/history/events", strings.NewReader(validEventJSON()))
			request.Header.Set("Authorization", "Bearer token")
			response := httptest.NewRecorder()
			authenticated.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func validEventJSON() string {
	return `{"event_id":"` + uuid.NewString() + `","track_id":"` + uuid.NewString() + `","started_at":"2026-11-10T11:58:00Z","finished_at":"2026-11-10T12:00:00Z","listened_ms":90000,"context_type":"direct","device_type":"web"}`
}

type tokenStub struct {
	identity identity.Identity
	err      error
}

func (stub tokenStub) Verify(context.Context, string, time.Time) (identity.Identity, error) {
	if stub.err != nil {
		return identity.Identity{}, stub.err
	}
	if stub.identity.UserID == uuid.Nil {
		return identity.Identity{}, identity.ErrInvalidAccessToken
	}
	return stub.identity, nil
}

type historyStub struct {
	userID uuid.UUID
	input  history.RecordInput
	result history.RecordResult
	err    error
	calls  int
}

func (stub *historyStub) Record(_ context.Context, userID uuid.UUID, input history.RecordInput) (history.RecordResult, error) {
	stub.userID = userID
	stub.input = input
	stub.calls++
	return stub.result, stub.err
}
