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

type storageStub struct{ err error }

func (stub storageStub) Check(context.Context) error { return stub.err }

func TestHealthChecksDatabaseAndStorage(t *testing.T) {
	tests := []struct {
		name       string
		database   DatabaseCheck
		storage    StorageCheck
		wantStatus int
	}{
		{"available", databaseStub{}, storageStub{}, http.StatusOK},
		{"database unavailable", databaseStub{errors.New("database")}, storageStub{}, http.StatusServiceUnavailable},
		{"storage unavailable", databaseStub{}, storageStub{errors.New("storage")}, http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()
			NewRouter(Dependencies{Database: test.database, Storage: test.storage}).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}
