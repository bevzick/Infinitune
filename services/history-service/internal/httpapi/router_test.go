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

func TestHealth(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{{"available", nil, http.StatusOK}, {"unavailable", errors.New("down"), http.StatusServiceUnavailable}} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			NewRouter(databaseStub{err: test.err}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}
