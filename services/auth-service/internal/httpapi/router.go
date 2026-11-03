package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type HealthCheck interface {
	Ping(context.Context) error
}

type Registration interface {
	Register(context.Context, auth.RegisterInput) (auth.RegistrationResult, error)
}

type Login interface {
	Login(context.Context, auth.LoginInput) (auth.AccessToken, error)
}

type RefreshSessions interface {
	Create(context.Context, uuid.UUID) (session.IssuedRefreshToken, error)
	Find(context.Context, string) (session.RefreshSession, error)
	Rotate(context.Context, string) (session.IssuedRefreshToken, error)
	Revoke(context.Context, string) (bool, error)
}

type Dependencies struct {
	Database     HealthCheck
	Redis        HealthCheck
	Registration Registration
	Login        Login
	Sessions     RefreshSessions
	Logger       *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", health(dependencies.Database, dependencies.Redis))
	router.POST("/v1/auth/register", register(dependencies.Registration, dependencies.Logger))
	router.POST("/v1/auth/login", login(dependencies.Login, dependencies.Logger))

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
