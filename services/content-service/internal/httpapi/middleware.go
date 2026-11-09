package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/gin-gonic/gin"
)

const identityContextKey = "verified_identity"

func authenticate(tokens IdentityVerifier, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		values := c.Request.Header.Values("Authorization")
		if len(values) != 1 || tokens == nil {
			writeUnauthorized(c)
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeUnauthorized(c)
			return
		}
		verified, err := tokens.Verify(c.Request.Context(), parts[1], time.Now())
		if errors.Is(err, identity.ErrInvalidAccessToken) {
			writeUnauthorized(c)
			return
		}
		if err != nil {
			logger.Error("identity verification unavailable", "error", err)
			writeError(c, http.StatusServiceUnavailable, "identity_unavailable", "Identity verification is unavailable")
			return
		}
		c.Set(identityContextKey, verified)
		c.Next()
	}
}

func requestIdentity(c *gin.Context) identity.Identity {
	value, _ := c.Get(identityContextKey)
	verified, _ := value.(identity.Identity)
	return verified
}

func writeUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	writeError(c, http.StatusUnauthorized, "invalid_access_token", "Access token is invalid or expired")
}

func cors(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Vary", "Origin")
			if slices.Contains(origins, origin) {
				c.Header("Access-Control-Allow-Origin", origin)
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}
