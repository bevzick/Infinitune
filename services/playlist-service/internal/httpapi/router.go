package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/identity"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
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

type Playlists interface {
	Create(context.Context, uuid.UUID, playlist.CreateInput) (playlist.Playlist, error)
	ByID(context.Context, uuid.UUID, uuid.UUID) (playlist.Playlist, error)
	Update(context.Context, uuid.UUID, uuid.UUID, playlist.UpdateInput) (playlist.Playlist, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type PlaylistTracks interface {
	Tracks(context.Context, uuid.UUID, uuid.UUID, playlist.PageRequest) (playlist.TrackPage, error)
	Add(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (playlist.Track, error)
	Remove(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	Reorder(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) ([]playlist.Track, error)
}

type DynamicPlaylists interface {
	Create(context.Context, uuid.UUID, playlist.DynamicCreateInput) (playlist.DynamicPlaylist, error)
	Criteria(context.Context, uuid.UUID, uuid.UUID) (rules.RuleSet, error)
	ReplaceCriteria(context.Context, uuid.UUID, uuid.UUID, rules.CreateInput) (rules.RuleSet, error)
}

type DynamicTracks interface {
	Resolve(context.Context, uuid.UUID, uuid.UUID, playlist.PageRequest) (playlist.DynamicTrackPage, error)
}

type Dependencies struct {
	Database         DatabaseCheck
	Tokens           IdentityVerifier
	Playlists        Playlists
	Tracks           PlaylistTracks
	DynamicPlaylists DynamicPlaylists
	DynamicTracks    DynamicTracks
	Origins          []string
	Logger           *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(dependencies.Origins))
	router.GET("/health", health(dependencies.Database))

	read := router.Group("/v1/playlists")
	read.Use(optionalAuthenticate(dependencies.Tokens, dependencies.Logger))
	read.GET("/:playlist_id", getPlaylist(dependencies.Playlists, dependencies.Logger))
	read.GET("/:playlist_id/tracks", getTracks(dependencies.Tracks, dependencies.Logger))
	read.GET("/:playlist_id/resolved-tracks",
		resolveDynamicTracks(dependencies.DynamicTracks, dependencies.Logger))

	protected := router.Group("/v1/playlists")
	protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
	protected.POST("", createPlaylist(dependencies.Playlists, dependencies.Logger))
	protected.POST("/dynamic", createDynamicPlaylist(dependencies.DynamicPlaylists, dependencies.Logger))
	protected.PATCH("/:playlist_id", updatePlaylist(dependencies.Playlists, dependencies.Logger))
	protected.DELETE("/:playlist_id", deletePlaylist(dependencies.Playlists, dependencies.Logger))
	protected.GET("/:playlist_id/criteria",
		getDynamicCriteria(dependencies.DynamicPlaylists, dependencies.Logger))
	protected.PUT("/:playlist_id/criteria",
		replaceDynamicCriteria(dependencies.DynamicPlaylists, dependencies.Logger))
	protected.POST("/:playlist_id/tracks", addTrack(dependencies.Tracks, dependencies.Logger))
	protected.DELETE("/:playlist_id/tracks/:track_id", removeTrack(dependencies.Tracks, dependencies.Logger))
	protected.PUT("/:playlist_id/tracks/order", reorderTracks(dependencies.Tracks, dependencies.Logger))
	return router
}

func health(database DatabaseCheck) gin.HandlerFunc {
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
