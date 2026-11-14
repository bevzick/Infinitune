package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/user-service/internal/identity"
	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const healthcheckTimeout = 2 * time.Second

type HealthCheck interface {
	Ping(context.Context) error
}

type IdentityVerifier interface {
	Verify(context.Context, string, time.Time) (identity.Identity, error)
}

type Profiles interface {
	Create(context.Context, uuid.UUID, profile.CreateInput) (profile.Profile, error)
	Own(context.Context, uuid.UUID, uuid.UUID) (profile.Profile, error)
	Public(context.Context, uuid.UUID) (profile.PublicProfile, error)
	Update(context.Context, uuid.UUID, uuid.UUID, profile.UpdateInput) (profile.Profile, error)
	UpdateSettings(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		profile.SettingsUpdateInput,
	) (profile.Settings, error)
}

type Dependencies struct {
	Database HealthCheck
	Tokens   IdentityVerifier
	Profiles Profiles
	Origins  []string
	Logger   *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)

	dependencies.Logger = normalizeLogger(dependencies.Logger)
	router := gin.New()
	router.Use(observeRequests(dependencies.Logger), recoverPanics(dependencies.Logger), cors(dependencies.Origins))
	router.GET("/health", health(dependencies.Database))
	router.GET("/v1/users/profiles/:user_id", publicProfile(dependencies.Profiles, dependencies.Logger))

	protected := router.Group("/v1/users/me")
	protected.Use(authenticate(dependencies.Tokens, dependencies.Logger))
	protected.POST("/profile", createProfile(dependencies.Profiles, dependencies.Logger))
	protected.GET("/profile", ownProfile(dependencies.Profiles, dependencies.Logger))
	protected.PATCH("/profile", updateProfile(dependencies.Profiles, dependencies.Logger))
	protected.GET("/settings", ownSettings(dependencies.Profiles, dependencies.Logger))
	protected.PATCH("/settings", updateSettings(dependencies.Profiles, dependencies.Logger))

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
