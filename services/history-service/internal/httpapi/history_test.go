package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/bevzick/Infinitune/services/history-service/internal/identity"
	"github.com/google/uuid"
)

func TestListHistoryIsOwnerScopedFilteredAndPaginated(t *testing.T) {
	userID, trackID, eventID := uuid.New(), uuid.New(), uuid.New()
	started := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	next := history.Cursor{StartedAt: started, EventID: eventID}
	service := &historyStub{page: history.Page{Items: []history.Event{{ID: eventID, UserID: userID, TrackID: trackID, StartedAt: started, ListenedMS: 30000, ContextType: "direct", DeviceType: "web"}}, NextCursor: &next}}
	router := NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, History: service})
	request := httptest.NewRequest(http.MethodGet, "/v1/history/events?track_id="+trackID.String()+"&completed=false&limit=10", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.userID != userID || service.listRequest.Limit != 10 || service.listRequest.Filter.TrackID == nil || *service.listRequest.Filter.TrackID != trackID || strings.Contains(response.Body.String(), "user_id") || !strings.Contains(response.Body.String(), "next_cursor") {
		t.Fatalf("status=%d user=%s request=%+v body=%s", response.Code, service.userID, service.listRequest, response.Body.String())
	}
}

func TestListHistoryNeverAcceptsForeignUserTarget(t *testing.T) {
	service := &historyStub{}
	router := NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: uuid.New()}}, History: service})
	request := httptest.NewRequest(http.MethodGet, "/v1/history/events?user_id="+uuid.NewString(), nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, service.calls, response.Body.String())
	}
}

func TestClearHistoryUsesOwnerAndOptionalCutoff(t *testing.T) {
	userID := uuid.New()
	service := &historyStub{clearResult: history.ClearResult{Deleted: 3}}
	router := NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, History: service})
	request := httptest.NewRequest(http.MethodDelete, "/v1/history/events?before=2026-11-10T12%3A00%3A00Z", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.userID != userID || service.clearRequest.Before == nil || !strings.Contains(response.Body.String(), `"deleted":3`) {
		t.Fatalf("status=%d user=%s request=%+v body=%s", response.Code, service.userID, service.clearRequest, response.Body.String())
	}
}

func TestCursorRoundTripAndTamperRejection(t *testing.T) {
	want := history.Cursor{StartedAt: time.Now().UTC().Truncate(time.Microsecond), EventID: uuid.New()}
	encoded := encodeCursor(want)
	got, err := decodeCursor(encoded)
	if err != nil || got.EventID != want.EventID || !got.StartedAt.Equal(want.StartedAt) {
		t.Fatalf("decodeCursor()=%+v,%v", got, err)
	}
	if _, err = decodeCursor(encoded + "!"); err == nil {
		t.Fatal("tampered cursor accepted")
	}
}
