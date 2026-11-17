package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bevzick/Infinitune/services/media-service/internal/artistimage"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ArtistImages interface {
	Upload(context.Context, uuid.UUID, uuid.UUID, string, int64, io.Reader) (artistimage.Image, error)
	Find(context.Context, uuid.UUID) (artistimage.Image, error)
}

func uploadArtistImage(service ArtistImages, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			writeError(c, 503, "image_unavailable", "Artist image service is unavailable")
			return
		}
		creatorID, err := uuid.Parse(c.Param("creator_id"))
		if err != nil || creatorID == uuid.Nil {
			writeError(c, 422, "invalid_creator", "creator_id must be a UUID")
			return
		}
		if c.Request.ContentLength < 0 {
			writeError(c, http.StatusLengthRequired, "content_length_required", "Content-Length required")
			return
		}
		if c.Request.ContentLength > artistimage.MaximumBytes {
			writeError(c, http.StatusRequestEntityTooLarge, "image_too_large", "Artist image exceeds 10 MiB")
			return
		}
		identity, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, artistimage.MaximumBytes)
		image, err := service.Upload(c.Request.Context(), identity.UserID, creatorID,
			c.GetHeader("Content-Type"), c.Request.ContentLength, c.Request.Body)
		if err != nil {
			writeArtistImageError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"artist_id": image.CatalogArtistID.String(),
			"image_url": "/v1/media/artists/" + image.CatalogArtistID.String() + "/image",
		})
	}
}

func deliverArtistImage(service ArtistImages, objects ObjectStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil || objects == nil {
			writeError(c, 503, "image_unavailable", "Artist image service is unavailable")
			return
		}
		artistID, err := uuid.Parse(c.Param("artist_id"))
		if err != nil || artistID == uuid.Nil {
			writeError(c, 400, "invalid_artist", "artist_id must be a UUID")
			return
		}
		image, err := service.Find(c.Request.Context(), artistID)
		if err != nil {
			if errors.Is(err, artistimage.ErrNotFound) {
				writeError(c, 404, "image_not_found", "Artist image was not found")
			} else {
				writeError(c, 503, "image_unavailable", "Artist image is temporarily unavailable")
			}
			return
		}
		object, err := objects.Open(c.Request.Context(), image.ObjectKey)
		if err != nil {
			if errors.Is(err, storage.ErrObjectNotFound) {
				writeError(c, 404, "image_not_found", "Artist image was not found")
			} else {
				writeError(c, 503, "image_unavailable", "Artist image is temporarily unavailable")
			}
			return
		}
		defer object.Body.Close()
		c.Header("Content-Type", image.MIMEType)
		c.Header("Content-Length", strconv.FormatInt(object.Size, 10))
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, object.Body)
	}
}

func writeArtistImageError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case exceedsRequestLimit(err), errors.Is(err, artistimage.ErrTooLarge):
		writeError(c, 413, "image_too_large", "Artist image exceeds 10 MiB")
	case errors.Is(err, artistimage.ErrInvalid):
		writeError(c, 422, "invalid_image", "Image format, length or dimensions are invalid")
	case errors.Is(err, artistimage.ErrForbidden):
		writeError(c, 403, "forbidden", "Artist image cannot be changed by this user")
	case errors.Is(err, artistimage.ErrStorage), errors.Is(err, artistimage.ErrUnavailable):
		writeError(c, 503, "image_unavailable", "Artist image storage is temporarily unavailable")
	default:
		loggerWithRequest(c, logger).Error("artist image upload failed", "error", err)
		writeError(c, 500, "internal_error", "Artist image could not be uploaded")
	}
}
