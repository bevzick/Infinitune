package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/ingest"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maximumUploadBytes int64 = 500 * 1024 * 1024

type ingestResponse struct {
	MediaAssetID   string       `json:"media_asset_id"`
	ContentTrackID string       `json:"content_track_id"`
	Status         media.Status `json:"status"`
}

func ingestUpload(service UploadIngest, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			writeError(c, http.StatusServiceUnavailable, "ingest_unavailable", "Media ingest is unavailable")
			return
		}
		uploadID, err := uuid.Parse(c.Param("upload_id"))
		if err != nil || uploadID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_upload", "upload_id must be a UUID")
			return
		}
		if c.Request.ContentLength < 0 {
			writeError(c, http.StatusLengthRequired, "content_length_required", "Content-Length is required")
			return
		}
		if c.Request.ContentLength > maximumUploadBytes {
			writeError(c, http.StatusRequestEntityTooLarge, "upload_too_large", "Audio upload is too large")
			return
		}
		verified, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximumUploadBytes)
		asset, created, err := service.Ingest(
			c.Request.Context(), verified.UserID, uploadID,
			c.GetHeader("Content-Type"), c.Request.ContentLength, c.Request.Body,
		)
		if err != nil {
			writeIngestError(c, logger, err)
			return
		}
		statusCode := http.StatusOK
		if created {
			statusCode = http.StatusCreated
		}
		c.JSON(statusCode, ingestResponse{
			MediaAssetID: asset.ID.String(), ContentTrackID: asset.ContentTrackID.String(),
			Status: asset.Status,
		})
	}
}

func writeIngestError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, ingest.ErrInvalid), errors.Is(err, content.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_upload", err.Error())
	case errors.Is(err, content.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Upload session is not available")
	case errors.Is(err, content.ErrNotFound):
		writeError(c, http.StatusNotFound, "upload_not_found", "Upload session was not found")
	case errors.Is(err, ingest.ErrInProgress), errors.Is(err, content.ErrConflict):
		writeError(c, http.StatusConflict, "upload_in_progress", "Upload is already in progress")
	case errors.Is(err, content.ErrInvalidTransition), errors.Is(err, ingest.ErrInconsistent):
		writeError(c, http.StatusConflict, "invalid_upload_state", "Upload state does not allow ingest")
	case errors.Is(err, content.ErrUnavailable), errors.Is(err, ingest.ErrStorage):
		writeError(c, http.StatusServiceUnavailable, "ingest_unavailable", "Media ingest is temporarily unavailable")
	default:
		loggerWithRequest(c, logger).Error("media ingest failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Upload could not be completed")
	}
}
