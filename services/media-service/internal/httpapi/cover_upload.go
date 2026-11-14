package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/coveringest"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CoverUploadIngest interface {
	Ingest(ctx context.Context, actorID, uploadID uuid.UUID, contentType string, size int64, body io.Reader) (coveringest.Result, bool, error)
}

type coverUploadResponse struct {
	ReleaseID string         `json:"release_id"`
	Status    content.Status `json:"status"`
}

func ingestCoverUpload(service CoverUploadIngest, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			writeError(c, http.StatusServiceUnavailable, "ingest_unavailable", "Cover ingest is unavailable")
			return
		}
		uploadID, err := uuid.Parse(c.Param("upload_id"))
		if err != nil || uploadID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_upload", "upload_id must be a UUID")
			return
		}
		if c.Request.ContentLength < 0 {
			writeError(c, http.StatusLengthRequired, "content_length_required", "Content-Length required")
			return
		}
		if c.Request.ContentLength > coveringest.MaximumCoverBytes {
			writeError(c, http.StatusRequestEntityTooLarge, "cover_too_large", "Cover upload exceeds 10 MiB")
			return
		}
		identity, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, coveringest.MaximumCoverBytes)
		result, created, err := service.Ingest(c.Request.Context(), identity.UserID, uploadID, c.GetHeader("Content-Type"), c.Request.ContentLength, c.Request.Body)
		if err != nil {
			writeCoverUploadError(c, logger, err)
			return
		}
		code := http.StatusOK
		if created {
			code = http.StatusCreated
		}
		c.JSON(code, coverUploadResponse{ReleaseID: result.ReleaseID.String(), Status: result.Status})
	}
}
func writeCoverUploadError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case exceedsRequestLimit(err):
		writeError(c, http.StatusRequestEntityTooLarge, "cover_too_large", "Cover exceeds 10 MiB")
	case errors.Is(err, coveringest.ErrTooLarge):
		writeError(c, 413, "cover_too_large", "Cover exceeds 10 MiB")
	case errors.Is(err, coveringest.ErrInvalid), errors.Is(err, content.ErrInvalid):
		writeError(c, 422, "invalid_cover", "Image format, length or dimensions are invalid")
	case errors.Is(err, content.ErrForbidden):
		writeError(c, 403, "forbidden", "Cover session not available")
	case errors.Is(err, content.ErrNotFound):
		writeError(c, 404, "upload_not_found", "Cover session not found")
	case errors.Is(err, coveringest.ErrInProgress), errors.Is(err, content.ErrConflict):
		writeError(c, 409, "upload_in_progress", "Cover upload already in progress")
	case errors.Is(err, coveringest.ErrInvalidState), errors.Is(err, content.ErrInvalidTransition):
		writeError(c, 409, "invalid_upload_state", "Cover upload state is invalid")
	case errors.Is(err, coveringest.ErrStorage), errors.Is(err, content.ErrUnavailable):
		writeError(c, 503, "ingest_unavailable", "Cover storage is temporarily unavailable")
	default:
		loggerWithRequest(c, logger).Error("cover upload failed", "error", err)
		writeError(c, 500, "internal_error", "Cover upload could not be completed")
	}
}

func exceedsRequestLimit(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}
