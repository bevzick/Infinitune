package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maximumRegistrationBodySize = 16 * 1024

type registrationRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registrationResponse struct {
	ID            uuid.UUID          `json:"id"`
	Email         string             `json:"email"`
	Status        auth.AccountStatus `json:"status"`
	EmailVerified bool               `json:"email_verified"`
	CreatedAt     time.Time          `json:"created_at"`
}

func register(registration Registration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		request, ok := decodeRegistrationRequest(c)
		if !ok {
			return
		}

		result, err := registration.Register(c.Request.Context(), auth.RegisterInput{
			Email:    request.Email,
			Password: request.Password,
		})
		if err != nil {
			handleRegistrationError(c, logger, err)
			return
		}

		c.JSON(http.StatusCreated, registrationResponse{
			ID:            result.ID,
			Email:         result.Email,
			Status:        result.Status,
			EmailVerified: result.EmailVerified,
			CreatedAt:     result.CreatedAt,
		})
	}
}

func decodeRegistrationRequest(c *gin.Context) (registrationRequest, bool) {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(
			c,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json",
		)
		return registrationRequest{}, false
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximumRegistrationBodySize)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	var request registrationRequest
	if err = decoder.Decode(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Request body must contain valid JSON")
		return registrationRequest{}, false
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(c, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object")
		return registrationRequest{}, false
	}

	return request, true
}

func handleRegistrationError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidEmail):
		writeError(c, http.StatusBadRequest, "invalid_email", "Email is invalid")
	case errors.Is(err, auth.ErrInvalidPassword):
		writeError(
			c,
			http.StatusBadRequest,
			"invalid_password",
			"Password must contain between 12 and 128 characters",
		)
	case errors.Is(err, auth.ErrEmailAlreadyRegistered):
		writeError(c, http.StatusConflict, "email_already_registered", "Email is already registered")
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusGatewayTimeout, "request_timeout", "Request timed out")
	default:
		logger.ErrorContext(c.Request.Context(), "user registration failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
