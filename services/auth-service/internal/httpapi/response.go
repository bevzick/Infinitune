package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

const maximumJSONBodySize = 16 * 1024

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}

func decodeJSON(c *gin.Context, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(
			c,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json",
		)
		return false
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maximumJSONBodySize)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err = decoder.Decode(destination); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Request body must contain valid JSON")
		return false
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(c, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object")
		return false
	}

	return true
}
