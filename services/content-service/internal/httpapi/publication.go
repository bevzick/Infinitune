package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type releasePublicationResponse struct {
	ReleaseID      string         `json:"release_id"`
	Status         release.Status `json:"status"`
	CatalogAlbumID *string        `json:"catalog_album_id,omitempty"`
	PublishedAt    *time.Time     `json:"published_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func publishRelease(service ReleasePublications, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, err := uuid.Parse(c.Param("release_id"))
		if err != nil || releaseID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity,
				"invalid_release_publication", "release_id must be a UUID")
			return
		}
		result, err := service.Publish(c.Request.Context(), requestIdentity(c).UserID, releaseID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		var albumID *string
		if result.CatalogAlbumID != nil {
			value := result.CatalogAlbumID.String()
			albumID = &value
		}
		c.JSON(http.StatusOK, releasePublicationResponse{
			ReleaseID: result.ID.String(), Status: result.Status,
			CatalogAlbumID: albumID, PublishedAt: result.PublishedAt,
			UpdatedAt: result.UpdatedAt,
		})
	}
}
