package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/bevzick/Infinitune/services/media-service/internal/sourceimport"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maximumImportRequestBytes = 4096

type externalImportRequest struct {
	ContentTrackID string              `json:"content_track_id"`
	Provider       source.ProviderName `json:"provider"`
	ExternalID     string              `json:"external_id"`
}

type externalImportResponse struct {
	MediaAssetID   string       `json:"media_asset_id"`
	ContentTrackID string       `json:"content_track_id"`
	Status         media.Status `json:"status"`
}

func importExternal(service ExternalImport, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			writeError(c, http.StatusServiceUnavailable, "import_unavailable", "External import is unavailable")
			return
		}
		verified, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximumImportRequestBytes)
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		var request externalImportRequest
		if err := decoder.Decode(&request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(c, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object")
			return
		}
		trackID, err := uuid.Parse(request.ContentTrackID)
		if err != nil || trackID == uuid.Nil || !request.Provider.Supported() ||
			strings.TrimSpace(request.ExternalID) == "" {
			writeError(c, http.StatusUnprocessableEntity, "invalid_import", "Import source is invalid")
			return
		}
		asset, created, err := service.Import(c.Request.Context(), verified.UserID,
			trackID, request.Provider, request.ExternalID)
		if err != nil {
			writeExternalImportError(c, logger, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		c.JSON(status, externalImportResponse{
			MediaAssetID: asset.ID.String(), ContentTrackID: asset.ContentTrackID.String(),
			Status: asset.Status,
		})
	}
}

func writeExternalImportError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, sourceimport.ErrInvalid), errors.Is(err, source.ErrInvalid),
		errors.Is(err, source.ErrUnsupportedProvider), errors.Is(err, sourceimport.ErrLicense),
		errors.Is(err, content.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_import", err.Error())
	case errors.Is(err, sourceimport.ErrForbidden), errors.Is(err, content.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Track is not available for import")
	case errors.Is(err, source.ErrNotFound), errors.Is(err, content.ErrNotFound):
		writeError(c, http.StatusNotFound, "source_not_found", "External track was not found")
	case errors.Is(err, sourceimport.ErrConflict), errors.Is(err, source.ErrConflict):
		writeError(c, http.StatusConflict, "import_conflict", "Track already has a different source")
	case errors.Is(err, content.ErrInvalidTransition), errors.Is(err, content.ErrConflict):
		writeError(c, http.StatusConflict, "import_not_allowed", "Release track does not accept imports")
	case errors.Is(err, source.ErrProviderUnavailable), errors.Is(err, sourceimport.ErrStorage),
		errors.Is(err, content.ErrUnavailable):
		writeError(c, http.StatusServiceUnavailable, "import_unavailable", "External import is temporarily unavailable")
	default:
		loggerWithRequest(c, logger).Error("external media import failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "External import could not be completed")
	}
}
