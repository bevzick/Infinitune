package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/gin-gonic/gin"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

func writeDomainError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, playlist.ErrInvalidArgument):
		writeError(c, http.StatusUnprocessableEntity, "invalid_playlist", err.Error())
	case errors.Is(err, playlist.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Operation is forbidden")
	case errors.Is(err, playlist.ErrNotFound):
		writeError(c, http.StatusNotFound, "playlist_not_found", "Playlist was not found")
	case errors.Is(err, playlist.ErrTrackNotFound):
		writeError(c, http.StatusNotFound, "track_not_found", "Track was not found")
	case errors.Is(err, playlist.ErrTrackExists):
		writeError(c, http.StatusConflict, "track_exists", "Track is already in the playlist")
	case errors.Is(err, playlist.ErrManualOnly):
		writeError(c, http.StatusConflict, "manual_playlist_required",
			"Operation is supported only for manual playlists")
	case errors.Is(err, playlist.ErrCatalogUnavailable):
		writeError(c, http.StatusServiceUnavailable, "catalog_unavailable",
			"Catalog service is temporarily unavailable")
	default:
		logger.Error("playlist request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed")
	}
}
