package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/gin-gonic/gin"
)

const (
	refreshCookieName = "infinitune_refresh"
	csrfCookieName    = "infinitune_csrf"
	csrfHeaderName    = "X-CSRF-Token"
	refreshCookiePath = "/v1/auth"
	csrfCookiePath    = "/"
	csrfTokenBytes    = 32
)

func refresh(sessions Sessions, cookies SessionCookieConfig, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		preventCaching(c)

		rawToken, err := c.Cookie(refreshCookieName)
		if err != nil {
			clearSessionCookies(c, cookies)
			writeError(c, http.StatusUnauthorized, "invalid_session", "Refresh session is invalid")
			return
		}
		if !validCSRF(c) {
			writeError(c, http.StatusForbidden, "csrf_validation_failed", "CSRF validation failed")
			return
		}

		csrfToken, err := newCSRFToken()
		if err != nil {
			logger.ErrorContext(c.Request.Context(), "failed to generate CSRF token", "error", err)
			writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}
		tokens, err := sessions.Refresh(c.Request.Context(), rawToken)
		if err != nil {
			handleRefreshError(c, cookies, logger, err)
			return
		}

		setSessionCookies(c, tokens.RefreshToken, csrfToken, cookies)
		writeAccessToken(c, tokens.AccessToken)
	}
}

func logout(sessions Sessions, cookies SessionCookieConfig, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		preventCaching(c)

		rawToken, err := c.Cookie(refreshCookieName)
		if errors.Is(err, http.ErrNoCookie) {
			clearSessionCookies(c, cookies)
			c.Status(http.StatusNoContent)
			return
		}
		if err != nil {
			clearSessionCookies(c, cookies)
			c.Status(http.StatusNoContent)
			return
		}
		if !validCSRF(c) {
			writeError(c, http.StatusForbidden, "csrf_validation_failed", "CSRF validation failed")
			return
		}

		_, err = sessions.Logout(c.Request.Context(), rawToken)
		if errors.Is(err, session.ErrInvalidRefreshToken) {
			clearSessionCookies(c, cookies)
			c.Status(http.StatusNoContent)
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(c, http.StatusGatewayTimeout, "request_timeout", "Request timed out")
			return
		}
		if err != nil {
			logger.ErrorContext(c.Request.Context(), "user logout failed", "error", err)
			writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}

		clearSessionCookies(c, cookies)
		c.Status(http.StatusNoContent)
	}
}

func handleRefreshError(c *gin.Context, cookies SessionCookieConfig, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, session.ErrInvalidRefreshToken),
		errors.Is(err, session.ErrRefreshSessionNotFound),
		errors.Is(err, session.ErrRefreshTokenReuse):
		clearSessionCookies(c, cookies)
		writeError(c, http.StatusUnauthorized, "invalid_session", "Refresh session is invalid")
	case errors.Is(err, auth.ErrAccountUnavailable):
		clearSessionCookies(c, cookies)
		writeError(c, http.StatusForbidden, "account_unavailable", "Account is unavailable")
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusGatewayTimeout, "request_timeout", "Request timed out")
	default:
		logger.ErrorContext(c.Request.Context(), "refresh session failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}

func setSessionCookies(
	c *gin.Context,
	refreshToken session.IssuedRefreshToken,
	csrfToken string,
	config SessionCookieConfig,
) {
	maxAge := int(time.Until(refreshToken.ExpiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    refreshToken.Value,
		Path:     refreshCookiePath,
		Expires:  refreshToken.ExpiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   config.Secure,
		SameSite: config.SameSite,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     csrfCookiePath,
		Expires:  refreshToken.ExpiresAt,
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   config.Secure,
		SameSite: config.SameSite,
	})
}

func clearSessionCookies(c *gin.Context, config SessionCookieConfig) {
	for _, cookie := range []http.Cookie{
		{Name: refreshCookieName, Path: refreshCookiePath, HttpOnly: true},
		{Name: csrfCookieName, Path: csrfCookiePath, HttpOnly: false},
	} {
		cookie.Value = ""
		cookie.Expires = time.Unix(1, 0).UTC()
		cookie.MaxAge = -1
		cookie.Secure = config.Secure
		cookie.SameSite = config.SameSite
		http.SetCookie(c.Writer, &cookie)
	}
}

func newCSRFToken() (string, error) {
	value := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate CSRF token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validCSRF(c *gin.Context) bool {
	cookie, err := c.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	cookieValue, err := base64.RawURLEncoding.Strict().DecodeString(cookie)
	if err != nil || len(cookieValue) != csrfTokenBytes {
		return false
	}
	headerValue, err := base64.RawURLEncoding.Strict().DecodeString(c.GetHeader(csrfHeaderName))
	if err != nil || len(headerValue) != csrfTokenBytes {
		return false
	}

	return subtle.ConstantTimeCompare(cookieValue, headerValue) == 1
}

func preventCaching(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}
