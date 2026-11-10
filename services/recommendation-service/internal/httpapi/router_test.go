package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/identity"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	"github.com/google/uuid"
)

type databaseStub struct{ err error }

func (stub databaseStub) Ping(context.Context) error { return stub.err }

func TestHealthReflectsDatabaseAvailability(t *testing.T) {
	tests := []struct {
		name     string
		database DatabaseCheck
		status   int
	}{
		{name: "ready", database: databaseStub{}, status: http.StatusOK},
		{name: "database down", database: databaseStub{err: errors.New("down")}, status: http.StatusServiceUnavailable},
		{name: "missing database", status: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()
			NewRouter(Dependencies{Database: test.database}).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}

type identityStub struct {
	identity identity.Identity
	err      error
}

func (stub identityStub) Verify(context.Context, string, time.Time) (identity.Identity, error) {
	return stub.identity, stub.err
}

type recommendationStub struct {
	run     recommendation.Run
	err     error
	userID  uuid.UUID
	refresh bool
}

func (stub *recommendationStub) Get(_ context.Context, userID uuid.UUID, refresh bool) (recommendation.Run, error) {
	stub.userID, stub.refresh = userID, refresh
	return stub.run, stub.err
}

func TestRecommendationsUseAuthenticatedUserAndPaginate(t *testing.T) {
	userID, runID := uuid.New(), uuid.New()
	service := &recommendationStub{run: recommendation.Run{
		ID: runID, UserID: userID, AlgorithmVersion: "transparent-v1",
		GeneratedAt: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 11, 10, 13, 0, 0, 0, time.UTC),
		Items: []recommendation.Item{{TrackID: uuid.New(), Rank: 1, Score: 2, Reason: "liked artist"}, {TrackID: uuid.New(), Rank: 2, Score: 1, Reason: "preferred genre"}},
	}}
	request := httptest.NewRequest(http.MethodGet, "/v1/recommendations?limit=1&offset=1&refresh=true", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	NewRouter(Dependencies{Tokens: identityStub{identity: identity.Identity{UserID: userID}}, Recommendations: service}).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.userID != userID || !service.refresh {
		t.Fatalf("Get() user = %s, refresh = %t", service.userID, service.refresh)
	}
	var body struct {
		RunID uuid.UUID `json:"run_id"`
		Items []struct {
			Rank int `json:"rank"`
		} `json:"items"`
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.RunID != runID || len(body.Items) != 1 || body.Items[0].Rank != 2 || body.Total != 2 || body.Limit != 1 || body.Offset != 1 {
		t.Fatalf("response = %+v", body)
	}
}

func TestRecommendationsRequireIdentityAndValidateQuery(t *testing.T) {
	service := &recommendationStub{}
	tests := []struct {
		name, path, authorization string
		status                    int
	}{
		{name: "missing token", path: "/v1/recommendations", status: http.StatusUnauthorized},
		{name: "invalid limit", path: "/v1/recommendations?limit=101", authorization: "Bearer access-token", status: http.StatusBadRequest},
		{name: "unknown query", path: "/v1/recommendations?user_id=" + uuid.NewString(), authorization: "Bearer access-token", status: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			NewRouter(Dependencies{Tokens: identityStub{identity: identity.Identity{UserID: uuid.New()}}, Recommendations: service}).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}
