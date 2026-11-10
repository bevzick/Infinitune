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
			NewRouter(test.database).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
