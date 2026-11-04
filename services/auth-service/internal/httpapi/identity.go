package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type identityResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	Roles     []string  `json:"roles"`
	TokenID   uuid.UUID `json:"token_id"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

const identityContextKey = "verified_identity"

func authenticate(tokens IdentityTokens) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")

		accessToken, ok := bearerToken(c.Request.Header.Values("Authorization"))
		if !ok {
			writeUnauthorized(c)
			return
		}

		verifiedIdentity, err := tokens.VerifyAccessToken(accessToken, time.Now())
		if err != nil {
			writeUnauthorized(c)
			return
		}

		c.Set(identityContextKey, verifiedIdentity)
		c.Next()
	}
}

func identity() gin.HandlerFunc {
	return func(c *gin.Context) {
		verifiedIdentity, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}

		writeIdentity(c, verifiedIdentity)
	}
}

func jwks(tokens IdentityTokens) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, tokens.JSONWebKeySet())
	}
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

func writeUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	writeError(c, http.StatusUnauthorized, "invalid_access_token", "Access token is invalid or expired")
}

func requestIdentity(c *gin.Context) (auth.Identity, bool) {
	value, exists := c.Get(identityContextKey)
	if !exists {
		return auth.Identity{}, false
	}

	verifiedIdentity, ok := value.(auth.Identity)
	return verifiedIdentity, ok
}

func writeIdentity(c *gin.Context, verifiedIdentity auth.Identity) {
	c.JSON(http.StatusOK, identityResponse{
		UserID:    verifiedIdentity.UserID,
		Roles:     verifiedIdentity.Roles,
		TokenID:   verifiedIdentity.TokenID,
		IssuedAt:  verifiedIdentity.IssuedAt,
		ExpiresAt: verifiedIdentity.ExpiresAt,
	})
}
