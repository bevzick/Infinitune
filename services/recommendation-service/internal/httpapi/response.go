package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
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
	case errors.Is(err, preferencesignal.ErrPersonalizationDisabled):
		writeError(c, http.StatusConflict, "personalization_disabled", "Personalized recommendations are disabled")
	case errors.Is(err, recommendation.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_recommendation_request", "Recommendation request is invalid")
	default:
		loggerWithRequest(c, logger).Error("recommendation request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed")
	}
}
