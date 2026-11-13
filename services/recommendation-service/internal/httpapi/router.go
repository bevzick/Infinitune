package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/identity"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type DatabaseCheck interface{ Ping(context.Context) error }
type IdentityVerifier interface {
	Verify(context.Context, string, time.Time) (identity.Identity, error)
}
type Recommendation interface {
	Get(context.Context, uuid.UUID, bool) (recommendation.Run, error)
}

type Dependencies struct {
	Database        DatabaseCheck
	Tokens          IdentityVerifier
	Recommendations Recommendation
	Origins         []string
	Logger          *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	dependencies.Logger = normalizeLogger(dependencies.Logger)
	router := gin.New()
	router.Use(observeRequests(dependencies.Logger), recoverPanics(dependencies.Logger), cors(dependencies.Origins))
	router.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthcheckTimeout)
		defer cancel()
		if dependencies.Database == nil || dependencies.Database.Ping(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	if dependencies.Recommendations != nil {
		protected := router.Group("/v1/recommendations")
		protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
		protected.GET("", listRecommendations(dependencies.Recommendations, dependencies.Logger))
	}
	return router
}
