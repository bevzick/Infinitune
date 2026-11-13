package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
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
	case errors.Is(err, history.ErrInvalidArgument):
		writeError(c, http.StatusUnprocessableEntity, "invalid_playback_event", err.Error())
	case errors.Is(err, history.ErrTrackNotFound):
		writeError(c, http.StatusNotFound, "track_not_found", "Track was not found")
	case errors.Is(err, history.ErrEventConflict):
		writeError(c, http.StatusConflict, "event_conflict", "Event identifier already belongs to different playback data")
	case errors.Is(err, history.ErrCatalogTimeout):
		writeError(c, http.StatusGatewayTimeout, "catalog_timeout", "Catalog service did not respond in time")
	case errors.Is(err, history.ErrCatalogUnavailable):
		writeError(c, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog service is temporarily unavailable")
	default:
		loggerWithRequest(c, logger).Error("history request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed")
	}
}
