package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/token"
	"github.com/gin-gonic/gin"
)

const healthcheckTimeout = 2 * time.Second

type HealthCheck interface {
	Ping(context.Context) error
}

type Registration interface {
	Register(context.Context, auth.RegisterInput) (auth.RegistrationResult, error)
}

type Login interface {
	Login(context.Context, auth.LoginInput) (auth.SessionTokens, error)
}

type Sessions interface {
	Refresh(context.Context, string) (auth.SessionTokens, error)
	Logout(context.Context, string) (bool, error)
}

type IdentityTokens interface {
	VerifyAccessToken(string, time.Time) (auth.Identity, error)
	JSONWebKeySet() token.JSONWebKeySet
}

type SessionCookieConfig struct {
	Secure   bool
	SameSite http.SameSite
}

type Dependencies struct {
	Database     HealthCheck
	Redis        HealthCheck
	Registration Registration
	Login        Login
	Sessions     Sessions
	Tokens       IdentityTokens
	Cookies      SessionCookieConfig
	Origins      []string
	Logger       *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(dependencies.Origins))
	router.GET("/health", health(dependencies.Database, dependencies.Redis))
	router.POST("/v1/auth/register", register(dependencies.Registration, dependencies.Logger))
	router.POST("/v1/auth/login", login(dependencies.Login, dependencies.Cookies, dependencies.Logger))
	router.POST("/v1/auth/refresh", refresh(dependencies.Sessions, dependencies.Cookies, dependencies.Logger))
	router.POST("/v1/auth/logout", logout(dependencies.Sessions, dependencies.Cookies, dependencies.Logger))
	router.GET("/v1/auth/jwks", jwks(dependencies.Tokens))
	router.GET("/v1/auth/identity", identity(dependencies.Tokens))

	return router
}

func health(checks ...HealthCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthcheckTimeout)
		defer cancel()

		for _, check := range checks {
			if err := check.Ping(ctx); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
