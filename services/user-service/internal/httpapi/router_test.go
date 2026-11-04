package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type healthCheckStub struct {
	err error
}

func (stub healthCheckStub) Ping(context.Context) error {
	return stub.err
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		database   HealthCheck
		wantStatus int
		wantBody   string
	}{
		{
			name:       "database available",
			database:   healthCheckStub{},
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "database unavailable",
			database:   healthCheckStub{err: errors.New("database unavailable")},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"status":"unavailable"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()

			NewRouter(test.database).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Body.String() != test.wantBody {
				t.Errorf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
		})
	}
}
