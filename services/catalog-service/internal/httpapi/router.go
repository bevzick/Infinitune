package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type HealthCheck interface {
	Ping(context.Context) error
}

type Catalog interface {
	Artist(context.Context, uuid.UUID) (metadata.Artist, error)
	Artists(context.Context, metadata.PageRequest) (metadata.Page[metadata.Artist], error)
	Album(context.Context, uuid.UUID) (metadata.Album, error)
	Albums(context.Context, metadata.PageRequest) (metadata.Page[metadata.Album], error)
	Track(context.Context, uuid.UUID) (metadata.Track, error)
	Tracks(context.Context, metadata.PageRequest) (metadata.Page[metadata.Track], error)
	Genre(context.Context, uuid.UUID) (metadata.Genre, error)
	Genres(context.Context, metadata.PageRequest) (metadata.Page[metadata.Genre], error)
	Mood(context.Context, uuid.UUID) (metadata.Mood, error)
	Moods(context.Context, metadata.PageRequest) (metadata.Page[metadata.Mood], error)
}

type Dependencies struct {
	Database HealthCheck
	Catalog  Catalog
	Origins  []string
	Logger   *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	if dependencies.Logger == nil {
		dependencies.Logger = slog.Default()
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(dependencies.Origins))
	router.GET("/health", health(dependencies.Database))

	catalog := router.Group("/v1/catalog")
	catalog.GET("/artists", listHandler(dependencies.Logger, dependencies.Catalog.Artists, artistResponseFrom))
	catalog.GET("/artists/:id", detailHandler(dependencies.Logger, dependencies.Catalog.Artist, artistResponseFrom))
	catalog.GET("/albums", listHandler(dependencies.Logger, dependencies.Catalog.Albums, albumResponseFrom))
	catalog.GET("/albums/:id", detailHandler(dependencies.Logger, dependencies.Catalog.Album, albumResponseFrom))
	catalog.GET("/tracks", listHandler(dependencies.Logger, dependencies.Catalog.Tracks, trackResponseFrom))
	catalog.GET("/tracks/:id", detailHandler(dependencies.Logger, dependencies.Catalog.Track, trackResponseFrom))
	catalog.GET("/genres", listHandler(dependencies.Logger, dependencies.Catalog.Genres, genreResponseFrom))
	catalog.GET("/genres/:id", detailHandler(dependencies.Logger, dependencies.Catalog.Genre, genreResponseFrom))
	catalog.GET("/moods", listHandler(dependencies.Logger, dependencies.Catalog.Moods, moodResponseFrom))
	catalog.GET("/moods/:id", detailHandler(dependencies.Logger, dependencies.Catalog.Mood, moodResponseFrom))

	return router
}

func health(database HealthCheck) gin.HandlerFunc {
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
