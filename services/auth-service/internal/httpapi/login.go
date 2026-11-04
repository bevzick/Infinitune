package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func login(loginService Login, cookies SessionCookieConfig, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")

		var request loginRequest
		if !decodeJSON(c, &request) {
			return
		}

		csrfToken, err := newCSRFToken()
		if err != nil {
			logger.ErrorContext(c.Request.Context(), "failed to generate CSRF token", "error", err)
			writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}

		tokens, err := loginService.Login(c.Request.Context(), auth.LoginInput{
			Email:    request.Email,
			Password: request.Password,
		})
		if err != nil {
			handleLoginError(c, logger, err)
			return
		}

		setSessionCookies(c, tokens.RefreshToken, csrfToken, cookies)
		writeAccessToken(c, tokens.AccessToken)
	}
}

func writeAccessToken(c *gin.Context, accessToken auth.AccessToken) {
	c.JSON(http.StatusOK, loginResponse{
		AccessToken: accessToken.Value,
		TokenType:   accessToken.Type,
		ExpiresAt:   accessToken.ExpiresAt,
	})
}

func handleLoginError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(c, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect")
	case errors.Is(err, auth.ErrAccountUnavailable):
		writeError(c, http.StatusForbidden, "account_unavailable", "Account is unavailable")
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusGatewayTimeout, "request_timeout", "Request timed out")
	default:
		logger.ErrorContext(c.Request.Context(), "user login failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
