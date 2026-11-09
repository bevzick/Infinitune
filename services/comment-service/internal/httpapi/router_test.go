package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type databaseStub struct{ err error }

func (stub databaseStub) Ping(context.Context) error { return stub.err }

func TestHealthReflectsDatabaseAvailability(t *testing.T) {
	tests := []struct {
		name       string
		database   DatabaseCheck
		wantStatus int
	}{
		{name: "available", database: databaseStub{}, wantStatus: http.StatusOK},
		{name: "unavailable", database: databaseStub{err: errors.New("database")},
			wantStatus: http.StatusServiceUnavailable},
		{name: "missing", database: nil, wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()
			NewRouter(Dependencies{Database: test.database}).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s",
					response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}
