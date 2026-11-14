package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/gin-gonic/gin"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeError(c *gin.Context, status int, code, message, field string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: errorBody{
		Code: code, Message: message, Field: field,
	}})
}

func writeDomainError(c *gin.Context, logger *slog.Logger, err error) {
	var validationError *metadata.ValidationError
	switch {
	case errors.As(err, &validationError):
		writeError(c, http.StatusBadRequest, "invalid_request", validationError.Reason, validationError.Field)
	case errors.Is(err, metadata.ErrNotFound):
		writeError(c, http.StatusNotFound, "catalog_item_not_found", "Catalog item was not found", "")
	default:
		loggerWithRequest(c, logger).Error("catalog request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed", "")
	}
}
