package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func playback(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		assetID, err := uuid.Parse(c.Param("media_asset_id"))
		if err != nil || assetID == uuid.Nil {
			writeError(c, http.StatusBadRequest, "invalid_media_asset_id", "Media asset ID must be a UUID")
			return
		}
		asset, variant, err := dependencies.Media.PlaybackVariant(c.Request.Context(), assetID)
		if err != nil {
			writePlaybackError(c, err)
			return
		}
		object, err := dependencies.Objects.Open(c.Request.Context(), variant.ObjectKey)
		if err != nil {
			identity, _ := requestIdentity(c)
			dependencies.Logger.Error("media object unavailable", "error", err,
				"media_asset_id", asset.ID, "variant_id", variant.ID, "user_id", identity.UserID)
			writeError(c, http.StatusServiceUnavailable, "media_unavailable", "Media is temporarily unavailable")
			return
		}
		defer object.Body.Close()

		contentType := object.ContentType
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(variant.ObjectKey))
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		c.Header("Content-Type", contentType)
		c.Header("Content-Length", strconv.FormatInt(object.Size, 10))
		c.Header("Cache-Control", "private, no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Status(http.StatusOK)
		if _, err = io.Copy(c.Writer, object.Body); err != nil && !errors.Is(err, c.Request.Context().Err()) {
			dependencies.Logger.Error("media stream interrupted", "error", err,
				"media_asset_id", asset.ID, "variant_id", variant.ID)
		}
	}
}

func writePlaybackError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, media.ErrInvalid):
		writeError(c, http.StatusBadRequest, "invalid_media_asset_id", "Media asset ID is invalid")
	case errors.Is(err, media.ErrNotFound):
		writeError(c, http.StatusNotFound, "media_not_found", "Media asset was not found")
	case errors.Is(err, media.ErrNotReady):
		writeError(c, http.StatusConflict, "media_not_ready", "Media asset is not ready for playback")
	case errors.Is(err, storage.ErrObjectNotFound):
		writeError(c, http.StatusServiceUnavailable, "media_unavailable", "Media is temporarily unavailable")
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "Media playback failed")
	}
}
