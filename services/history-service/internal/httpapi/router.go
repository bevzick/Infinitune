package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/bevzick/Infinitune/services/history-service/internal/identity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type DatabaseCheck interface{ Ping(context.Context) error }
type IdentityVerifier interface {
	Verify(context.Context, string, time.Time) (identity.Identity, error)
}
type History interface {
	Record(context.Context, uuid.UUID, history.RecordInput) (history.RecordResult, error)
	List(context.Context, uuid.UUID, history.ListRequest) (history.Page, error)
	Clear(context.Context, uuid.UUID, history.ClearRequest) (history.ClearResult, error)
}

type Dependencies struct {
	Database DatabaseCheck
	Tokens   IdentityVerifier
	History  History
	Origins  []string
	Logger   *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(dependencies.Origins))
	router.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthcheckTimeout)
		defer cancel()
		if dependencies.Database == nil || dependencies.Database.Ping(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	if dependencies.History != nil {
		protected := router.Group("/v1/history")
		protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
		protected.POST("/events", recordEvent(dependencies.History, dependencies.Logger))
		protected.GET("/events", listEvents(dependencies.History, dependencies.Logger))
		protected.DELETE("/events", clearEvents(dependencies.History, dependencies.Logger))
	}
	return router
}
