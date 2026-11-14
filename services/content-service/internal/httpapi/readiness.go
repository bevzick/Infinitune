package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// getReleaseReadiness does not mutate the release. Submit always revalidates.
func getReleaseReadiness(service ReleaseReadiness, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, err := uuid.Parse(c.Param("release_id"))
		if err != nil || releaseID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity,
				"invalid_release_submission", "release_id must be a UUID")
			return
		}
		result, err := service.Readiness(
			c.Request.Context(), requestIdentity(c).UserID, releaseID,
		)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
