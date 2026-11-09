package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type DatabaseCheck interface{ Ping(context.Context) error }
type IdentityVerifier interface {
	Verify(context.Context, string, time.Time) (identity.Identity, error)
}
type CreatorAccounts interface {
	Create(context.Context, uuid.UUID, creator.CreateInput) (creator.Account, error)
	List(context.Context, uuid.UUID) ([]creator.Account, error)
	ByID(context.Context, uuid.UUID, uuid.UUID) (creator.Account, error)
	Update(context.Context, uuid.UUID, uuid.UUID, creator.UpdateInput) (creator.Account, error)
	SubmitVerification(context.Context, uuid.UUID, uuid.UUID) (creator.Account, error)
	ReviewVerification(context.Context, creator.Actor, uuid.UUID, creator.VerificationStatus) (creator.Account, error)
}
type CreatorMembers interface {
	List(context.Context, uuid.UUID, uuid.UUID) ([]creator.Member, error)
	Add(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, creator.MemberRole) (creator.Member, error)
	ChangeRole(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, creator.MemberRole) (creator.Member, error)
	Remove(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}
type TrackRights interface {
	Set(context.Context, uuid.UUID, rights.SetInput) (rights.Right, error)
	List(context.Context, uuid.UUID, uuid.UUID) ([]rights.Right, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}
type ReleaseSubmissions interface {
	Submit(context.Context, uuid.UUID, uuid.UUID) (release.Release, error)
}
type ReleaseModeration interface {
	Review(context.Context, creator.Actor, moderation.Input) (moderation.Review, error)
}
type ReleasePublications interface {
	Publish(context.Context, uuid.UUID, uuid.UUID) (release.Release, error)
}
type Releases interface {
	CreateDraft(context.Context, uuid.UUID, release.CreateDraftInput) (release.Draft, error)
	ByID(context.Context, uuid.UUID, uuid.UUID) (release.Draft, error)
	Update(context.Context, uuid.UUID, uuid.UUID, release.UpdateInput) (release.Draft, error)
	Withdraw(context.Context, uuid.UUID, uuid.UUID) (release.Release, error)
	Archive(context.Context, uuid.UUID, uuid.UUID) (release.Release, error)
}
type Dependencies struct {
	Database     DatabaseCheck
	Tokens       IdentityVerifier
	Creators     CreatorAccounts
	Members      CreatorMembers
	Rights       TrackRights
	Submissions  ReleaseSubmissions
	Moderation   ReleaseModeration
	Publications ReleasePublications
	Releases     Releases
	Origins      []string
	Logger       *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(dependencies.Origins))
	router.GET("/health", health(dependencies.Database))
	protected := router.Group("/v1/creators")
	protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
	protected.POST("", createCreator(dependencies.Creators, dependencies.Logger))
	protected.GET("", listCreators(dependencies.Creators, dependencies.Logger))
	protected.GET("/:creator_id", getCreator(dependencies.Creators, dependencies.Logger))
	protected.PATCH("/:creator_id", updateCreator(dependencies.Creators, dependencies.Logger))
	protected.POST("/:creator_id/verification-submissions", submitVerification(dependencies.Creators, dependencies.Logger))
	protected.POST("/:creator_id/verification-reviews", reviewVerification(dependencies.Creators, dependencies.Logger))
	protected.GET("/:creator_id/members", listMembers(dependencies.Members, dependencies.Logger))
	protected.POST("/:creator_id/members", addMember(dependencies.Members, dependencies.Logger))
	protected.PATCH("/:creator_id/members/:user_id", changeMemberRole(dependencies.Members, dependencies.Logger))
	protected.DELETE("/:creator_id/members/:user_id", removeMember(dependencies.Members, dependencies.Logger))
	content := router.Group("/v1")
	content.Use(authenticate(dependencies.Tokens, dependencies.Logger))
	content.PUT("/release-tracks/:release_track_id/rights", setTrackRights(dependencies.Rights, dependencies.Logger))
	content.GET("/release-tracks/:release_track_id/rights", listTrackRights(dependencies.Rights, dependencies.Logger))
	content.DELETE("/track-rights/:right_id", deleteTrackRights(dependencies.Rights, dependencies.Logger))
	content.POST("/releases", createRelease(dependencies.Releases, dependencies.Logger))
	content.GET("/releases/:release_id", getRelease(dependencies.Releases, dependencies.Logger))
	content.PUT("/releases/:release_id", updateRelease(dependencies.Releases, dependencies.Logger))
	content.POST("/releases/:release_id/withdrawals", withdrawRelease(dependencies.Releases, dependencies.Logger))
	content.DELETE("/releases/:release_id", archiveRelease(dependencies.Releases, dependencies.Logger))
	content.POST("/releases/:release_id/submissions", submitRelease(dependencies.Submissions, dependencies.Logger))
	content.POST("/releases/:release_id/moderation-reviews", reviewRelease(dependencies.Moderation, dependencies.Logger))
	content.POST("/releases/:release_id/publications", publishRelease(dependencies.Publications, dependencies.Logger))
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
