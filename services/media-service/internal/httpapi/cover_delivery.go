package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PublishedCover interface {
	Published(context.Context, uuid.UUID) (content.CoverSession, error)
}

func deliverCover(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, err := uuid.Parse(c.Param("release_id"))
		if err != nil || releaseID == uuid.Nil {
			writeError(c, http.StatusBadRequest, "invalid_release_id", "Release ID must be a UUID")
			return
		}
		session, err := dependencies.PublishedCovers.Published(c.Request.Context(), releaseID)
		if err != nil {
			if errors.Is(err, content.ErrNotFound) || errors.Is(err, content.ErrForbidden) {
				writeError(c, http.StatusNotFound, "cover_not_found", "Cover was not found")
			} else {
				writeError(c, http.StatusServiceUnavailable, "cover_unavailable", "Cover is temporarily unavailable")
			}
			return
		}
		object, err := dependencies.Objects.Open(c.Request.Context(), session.ObjectKey)
		if err != nil {
			if errors.Is(err, storage.ErrObjectNotFound) {
				writeError(c, http.StatusNotFound, "cover_not_found", "Cover was not found")
			} else {
				writeError(c, http.StatusServiceUnavailable, "cover_unavailable", "Cover is temporarily unavailable")
			}
			return
		}
		defer object.Body.Close()
		c.Header("Content-Type", session.MIMEType)
		c.Header("Content-Length", strconv.FormatInt(object.Size, 10))
		c.Header("Cache-Control", "public, max-age=86400, immutable")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, object.Body)
	}
}
