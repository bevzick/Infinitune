package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/identity"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
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

type Likes interface {
	LikeTrack(context.Context, uuid.UUID, uuid.UUID) (interaction.Like, error)
	UnlikeTrack(context.Context, uuid.UUID, uuid.UUID) error
	TrackLikes(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error)
	LikeAlbum(context.Context, uuid.UUID, uuid.UUID) (interaction.Like, error)
	UnlikeAlbum(context.Context, uuid.UUID, uuid.UUID) error
	AlbumLikes(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error)
	LikeComment(context.Context, uuid.UUID, uuid.UUID) (interaction.Like, error)
	UnlikeComment(context.Context, uuid.UUID, uuid.UUID) error
	CommentLikes(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error)
}

type Follows interface {
	FollowArtist(context.Context, uuid.UUID, uuid.UUID) (interaction.Follow, error)
	UnfollowArtist(context.Context, uuid.UUID, uuid.UUID) error
	ArtistFollows(context.Context, uuid.UUID, interaction.PageRequest) (interaction.FollowPage, error)
}

type Dependencies struct {
	Database DatabaseCheck
	Tokens   IdentityVerifier
	Likes    Likes
	Follows  Follows
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

	protected := router.Group("/v1/interactions")
	protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
	if dependencies.Likes != nil {
		protected.GET("/tracks/likes", listTrackLikes(dependencies.Likes, dependencies.Logger))
		protected.PUT("/tracks/:track_id/like", likeTrack(dependencies.Likes, dependencies.Logger))
		protected.DELETE("/tracks/:track_id/like", unlikeTrack(dependencies.Likes, dependencies.Logger))
		protected.GET("/albums/likes", listAlbumLikes(dependencies.Likes, dependencies.Logger))
		protected.PUT("/albums/:album_id/like", likeAlbum(dependencies.Likes, dependencies.Logger))
		protected.DELETE("/albums/:album_id/like", unlikeAlbum(dependencies.Likes, dependencies.Logger))
		protected.GET("/comments/likes", listCommentLikes(dependencies.Likes, dependencies.Logger))
		protected.PUT("/comments/:comment_id/like", likeComment(dependencies.Likes, dependencies.Logger))
		protected.DELETE("/comments/:comment_id/like", unlikeComment(dependencies.Likes, dependencies.Logger))
	}
	if dependencies.Follows != nil {
		protected.GET("/artists/follows", listArtistFollows(dependencies.Follows, dependencies.Logger))
		protected.PUT("/artists/:artist_id/follow", followArtist(dependencies.Follows, dependencies.Logger))
		protected.DELETE("/artists/:artist_id/follow", unfollowArtist(dependencies.Follows, dependencies.Logger))
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
