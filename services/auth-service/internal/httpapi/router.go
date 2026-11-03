package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/gin-gonic/gin"
)

const healthcheckTimeout = 2 * time.Second

type DatabaseHealth interface {
	Ping(context.Context) error
}

type Registration interface {
	Register(context.Context, auth.RegisterInput) (auth.RegistrationResult, error)
}

type Dependencies struct {
	Database     DatabaseHealth
	Registration Registration
	Logger       *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", health(dependencies.Database))
	router.POST("/v1/auth/register", register(dependencies.Registration, dependencies.Logger))

	return router
}

func health(database DatabaseHealth) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthcheckTimeout)
		defer cancel()

		if err := database.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
