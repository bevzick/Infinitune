package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestObserveRequestsPreservesValidRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := gin.New()
	router.Use(observeRequests(logger))
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(requestIDHeader, "frontend-request_42")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if got := response.Header().Get(requestIDHeader); got != "frontend-request_42" {
		t.Fatalf("%s = %q, want frontend-request_42", requestIDHeader, got)
	}
	output := logs.String()
	for _, expected := range []string{
		"\"event\":\"http_request\"",
		"\"request_id\":\"frontend-request_42\"",
		"\"route\":\"/health\"",
		"\"status\":204",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("log output %q does not contain %q", output, expected)
		}
	}
}

func TestObserveRequestsReplacesInvalidRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(observeRequests(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))))
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(requestIDHeader, "invalid request id")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if _, err := uuid.Parse(response.Header().Get(requestIDHeader)); err != nil {
		t.Fatalf("%s is not a generated UUID: %v", requestIDHeader, err)
	}
}

func TestRecoverPanicsReturnsStructuredErrorAndCorrelatedLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := gin.New()
	router.Use(observeRequests(logger), recoverPanics(logger))
	router.GET("/panic", func(*gin.Context) {
		panic("test panic")
	})

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	request.Header.Set(requestIDHeader, "panic-request")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(response.Body.String(), "\"code\":\"internal_error\"") {
		t.Fatalf("response body = %q, want structured internal error", response.Body.String())
	}
	output := logs.String()
	for _, expected := range []string{
		"\"event\":\"http_panic\"",
		"\"event\":\"http_request\"",
		"\"request_id\":\"panic-request\"",
		"\"status\":500",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("log output does not contain %q", expected)
		}
	}
}
