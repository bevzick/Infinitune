package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/identity"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type DatabaseCheck interface {
	Ping(context.Context) error
}

type StorageCheck interface {
	Check(context.Context) error
}

type IdentityVerifier interface {
	Verify(context.Context, string, time.Time) (identity.Identity, error)
}

type Playback interface {
	PlaybackVariant(context.Context, uuid.UUID, media.AudioQuality) (media.Asset, media.AudioVariant, error)
}

type ObjectStorage interface {
	Open(context.Context, string) (storage.Object, error)
}

type UploadIngest interface {
	Ingest(context.Context, uuid.UUID, uuid.UUID, string, int64, io.Reader) (media.Asset, bool, error)
}

type Dependencies struct {
	Database DatabaseCheck
	Storage  StorageCheck
	Objects  ObjectStorage
	Media    Playback
	Ingest   UploadIngest
	Tokens   IdentityVerifier
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
	router.GET("/health", health(dependencies))
	protected := router.Group("/v1/media")
	protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
	protected.GET("/assets/:media_asset_id/playback", playback(dependencies))
	protected.PUT("/uploads/:upload_id", ingestUpload(dependencies.Ingest, dependencies.Logger))
	return router
}

func health(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthcheckTimeout)
		defer cancel()
		if dependencies.Database.Ping(ctx) != nil || dependencies.Storage.Check(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
