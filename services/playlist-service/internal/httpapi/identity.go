package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/identity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const identityContextKey = "verified_identity"

func authenticate(tokens IdentityVerifier, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		accessToken, ok := bearerToken(c.Request.Header.Values("Authorization"))
		if !ok || tokens == nil {
			writeUnauthorized(c)
			return
		}
		if !verifyIdentity(c, tokens, logger, accessToken) {
			return
		}
		c.Next()
	}
}

func optionalAuthenticate(tokens IdentityVerifier, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Authorization")
		if len(values) == 0 {
			c.Next()
			return
		}
		c.Header("Cache-Control", "no-store")
		accessToken, ok := bearerToken(values)
		if !ok || tokens == nil {
			writeUnauthorized(c)
			return
		}
		if !verifyIdentity(c, tokens, logger, accessToken) {
			return
		}
		c.Next()
	}
}

func verifyIdentity(
	c *gin.Context,
	tokens IdentityVerifier,
	logger *slog.Logger,
	accessToken string,
) bool {
	verified, err := tokens.Verify(c.Request.Context(), accessToken, time.Now())
	if errors.Is(err, identity.ErrInvalidAccessToken) {
		writeUnauthorized(c)
		return false
	}
	if err != nil {
		loggerWithRequest(c, logger).Error("identity verification unavailable", "error", err)
		writeError(c, http.StatusServiceUnavailable,
			"identity_unavailable", "Identity verification is unavailable")
		return false
	}
	c.Set(identityContextKey, verified)
	return true
}

func bearerToken(values []string) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func requestIdentity(c *gin.Context) (identity.Identity, bool) {
	value, exists := c.Get(identityContextKey)
	if !exists {
		return identity.Identity{}, false
	}
	verified, ok := value.(identity.Identity)
	return verified, ok
}

func viewerID(c *gin.Context) uuid.UUID {
	verified, ok := requestIdentity(c)
	if !ok {
		return uuid.Nil
	}
	return verified.UserID
}

func writeUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	writeError(c, http.StatusUnauthorized, "invalid_access_token", "Access token is invalid or expired")
}
