package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
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

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

func writeDomainError(c *gin.Context, logger *slog.Logger, err error) {
	var validationError *profile.ValidationError
	switch {
	case errors.As(err, &validationError):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, errorResponse{Error: errorBody{
			Code: "invalid_profile", Message: validationError.Reason, Field: validationError.Field,
		}})
	case errors.Is(err, profile.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Operation is forbidden")
	case errors.Is(err, profile.ErrNotFound):
		writeError(c, http.StatusNotFound, "profile_not_found", "Profile was not found")
	case errors.Is(err, profile.ErrAlreadyExists):
		writeError(c, http.StatusConflict, "profile_exists", "Profile already exists")
	case errors.Is(err, profile.ErrUsernameTaken):
		writeError(c, http.StatusConflict, "username_taken", "Username is already taken")
	default:
		loggerWithRequest(c, logger).Error("user request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed")
	}
}
