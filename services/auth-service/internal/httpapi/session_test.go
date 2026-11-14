package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestSessionCookiesUseSeparateSecurityAttributes(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	expiresAt := time.Now().Add(time.Hour).UTC()

	setSessionCookies(context, session.IssuedRefreshToken{
		Value:     "refresh-token",
		SessionID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		UserID:    uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		ExpiresAt: expiresAt,
	}, "csrf-token", SessionCookieConfig{
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookie count = %d, want 2", len(cookies))
	}
	refreshCookie := findCookie(t, cookies, refreshCookieName)
	if !refreshCookie.HttpOnly || !refreshCookie.Secure || refreshCookie.Path != refreshCookiePath ||
		refreshCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("refresh cookie = %+v", refreshCookie)
	}
	csrfCookie := findCookie(t, cookies, csrfCookieName)
	if csrfCookie.HttpOnly || !csrfCookie.Secure || csrfCookie.Path != csrfCookiePath ||
		csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("CSRF cookie = %+v", csrfCookie)
	}
}

func TestRefreshRejectsMismatchedCSRFBeforeRotatingSession(t *testing.T) {
	sessions := &sessionsHandlerFake{}
	router := gin.New()
	router.POST("/refresh", refresh(
		sessions,
		SessionCookieConfig{SameSite: http.SameSiteLaxMode},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	))

	request := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-token"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfTestToken(0x11)})
	request.Header.Set(csrfHeaderName, csrfTestToken(0x22))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("POST refresh status = %d, body = %s", response.Code, response.Body.String())
	}
	if sessions.refreshCalls != 0 {
		t.Fatalf("Refresh() calls = %d", sessions.refreshCalls)
	}
}

func TestRefreshRotatesCookiesWithValidCSRF(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).UTC()
	sessions := &sessionsHandlerFake{tokens: auth.SessionTokens{
		AccessToken: auth.AccessToken{
			Value:     "new-access-token",
			Type:      "Bearer",
			ExpiresAt: time.Now().Add(time.Minute),
		},
		RefreshToken: session.IssuedRefreshToken{
			Value:     "new-refresh-token",
			SessionID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			UserID:    uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			ExpiresAt: expiresAt,
		},
	}}
	router := gin.New()
	router.POST("/refresh", refresh(
		sessions,
		SessionCookieConfig{SameSite: http.SameSiteLaxMode},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	))

	csrfToken := csrfTestToken(0x33)
	request := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "old-refresh-token"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	request.Header.Set(csrfHeaderName, csrfToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || sessions.refreshCalls != 1 || sessions.refreshToken != "old-refresh-token" {
		t.Fatalf(
			"POST refresh status = %d, calls = %d, token = %q, body = %s",
			response.Code,
			sessions.refreshCalls,
			sessions.refreshToken,
			response.Body.String(),
		)
	}
	refreshCookie := findCookie(t, response.Result().Cookies(), refreshCookieName)
	if refreshCookie.Value != "new-refresh-token" || !refreshCookie.HttpOnly {
		t.Fatalf("refresh cookie = %+v", refreshCookie)
	}
	csrfCookie := findCookie(t, response.Result().Cookies(), csrfCookieName)
	if csrfCookie.Value == "" || csrfCookie.Value == csrfToken || csrfCookie.HttpOnly {
		t.Fatalf("CSRF cookie = %+v", csrfCookie)
	}
}

func TestLogoutWithoutSessionIsIdempotent(t *testing.T) {
	sessions := &sessionsHandlerFake{}
	router := gin.New()
	router.POST("/logout", logout(
		sessions,
		SessionCookieConfig{SameSite: http.SameSiteLaxMode},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	))

	request := httptest.NewRequest(http.MethodPost, "/logout", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || sessions.logoutCalls != 0 {
		t.Fatalf("POST logout status = %d, calls = %d", response.Code, sessions.logoutCalls)
	}
	for _, cookieName := range []string{refreshCookieName, csrfCookieName} {
		cookie := findCookie(t, response.Result().Cookies(), cookieName)
		if cookie.MaxAge != -1 {
			t.Errorf("cleared cookie %q MaxAge = %d", cookieName, cookie.MaxAge)
		}
	}
}

type sessionsHandlerFake struct {
	tokens       auth.SessionTokens
	err          error
	refreshCalls int
	logoutCalls  int
	refreshToken string
}

func (s *sessionsHandlerFake) Refresh(_ context.Context, rawToken string) (auth.SessionTokens, error) {
	s.refreshCalls++
	s.refreshToken = rawToken
	return s.tokens, s.err
}

func (s *sessionsHandlerFake) Logout(_ context.Context, rawToken string) (bool, error) {
	s.logoutCalls++
	s.refreshToken = rawToken
	return true, s.err
}

func csrfTestToken(value byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{value}, csrfTokenBytes))
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q was not set", name)
	return nil
}
