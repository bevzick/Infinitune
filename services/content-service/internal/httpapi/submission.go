package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type releaseSubmissionResponse struct {
	ReleaseID   string         `json:"release_id"`
	Status      release.Status `json:"status"`
	SubmittedAt *time.Time     `json:"submitted_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func submitRelease(service ReleaseSubmissions, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, err := uuid.Parse(c.Param("release_id"))
		if err != nil || releaseID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity,
				"invalid_release_submission", "release_id must be a UUID")
			return
		}
		result, err := service.Submit(
			c.Request.Context(), requestIdentity(c).UserID, releaseID,
		)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, releaseSubmissionResponse{
			ReleaseID: result.ID.String(), Status: result.Status,
			SubmittedAt: result.SubmittedAt, UpdatedAt: result.UpdatedAt,
		})
	}
}
