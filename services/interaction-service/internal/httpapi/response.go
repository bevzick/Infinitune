package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
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
	case errors.Is(err, interaction.ErrInvalidArgument):
		writeError(c, http.StatusUnprocessableEntity, "invalid_interaction", err.Error())
	case errors.Is(err, interaction.ErrTrackNotFound):
		writeError(c, http.StatusNotFound, "track_not_found", "Track was not found")
	case errors.Is(err, interaction.ErrAlbumNotFound):
		writeError(c, http.StatusNotFound, "album_not_found", "Album was not found")
	case errors.Is(err, interaction.ErrArtistNotFound):
		writeError(c, http.StatusNotFound, "artist_not_found", "Artist was not found")
	case errors.Is(err, interaction.ErrCommentNotFound):
		writeError(c, http.StatusNotFound, "comment_not_found", "Comment was not found")
	case errors.Is(err, interaction.ErrCatalogTimeout):
		writeError(c, http.StatusGatewayTimeout, "catalog_timeout", "Catalog service did not respond in time")
	case errors.Is(err, interaction.ErrCatalogUnavailable):
		writeError(c, http.StatusServiceUnavailable,
			"catalog_unavailable", "Catalog service is temporarily unavailable")
	case errors.Is(err, interaction.ErrCommentTimeout):
		writeError(c, http.StatusGatewayTimeout, "comment_timeout", "Comment service did not respond in time")
	case errors.Is(err, interaction.ErrCommentUnavailable):
		writeError(c, http.StatusServiceUnavailable,
			"comment_unavailable", "Comment service is temporarily unavailable")
	default:
		logger.Error("interaction request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed")
	}
}
