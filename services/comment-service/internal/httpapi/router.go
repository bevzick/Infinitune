package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/bevzick/Infinitune/services/comment-service/internal/identity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type DatabaseCheck interface {
	Ping(context.Context) error
}

type IdentityVerifier interface {
	Verify(context.Context, string, time.Time) (identity.Identity, error)
}
type Comments interface {
	Create(context.Context, uuid.UUID, comment.CreateInput) (comment.Comment, error)
	List(context.Context, uuid.UUID, comment.PageRequest) (comment.Page, error)
	Update(context.Context, uuid.UUID, uuid.UUID, string) (comment.Comment, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
	Report(context.Context, uuid.UUID, uuid.UUID, comment.ReportInput) (comment.Report, error)
}
type Dependencies struct {
	Database DatabaseCheck
	Tokens   IdentityVerifier
	Comments Comments
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
	router.GET("/health", health(dependencies.Database))
	if dependencies.Comments != nil {
		router.GET("/v1/tracks/:track_id/comments", listComments(dependencies.Comments, dependencies.Logger))
		protected := router.Group("/v1/comments")
		protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
		protected.POST("", createComment(dependencies.Comments, dependencies.Logger))
		protected.PATCH("/:comment_id", updateComment(dependencies.Comments, dependencies.Logger))
		protected.DELETE("/:comment_id", deleteComment(dependencies.Comments, dependencies.Logger))
		protected.POST("/:comment_id/reports", reportComment(dependencies.Comments, dependencies.Logger))
	}
	return router
}

func health(database DatabaseCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthcheckTimeout)
		defer cancel()
		if database == nil || database.Ping(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
