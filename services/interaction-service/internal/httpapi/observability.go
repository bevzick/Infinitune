package httpapi

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	requestIDHeader     = "X-Request-ID"
	requestIDContextKey = "request_id"
	maximumRequestIDLen = 128
)

func observeRequests(logger *slog.Logger) gin.HandlerFunc {
	logger = normalizeLogger(logger)
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}

		c.Set(requestIDContextKey, requestID)
		c.Header(requestIDHeader, requestID)

		startedAt := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		attributes := []any{
			"event", "http_request",
			"request_id", requestID,
			"method", c.Request.Method,
			"route", route,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"response_bytes", c.Writer.Size(),
			"client_ip", c.ClientIP(),
		}
		if len(c.Errors) > 0 {
			attributes = append(attributes, "error_count", len(c.Errors))
		}

		switch status := c.Writer.Status(); {
		case status >= http.StatusInternalServerError:
			logger.Error("HTTP request completed", attributes...)
		case status >= http.StatusBadRequest:
			logger.Warn("HTTP request completed", attributes...)
		default:
			logger.Info("HTTP request completed", attributes...)
		}
	}
}

func recoverPanics(logger *slog.Logger) gin.HandlerFunc {
	logger = normalizeLogger(logger)
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		loggerWithRequest(c, logger).Error(
			"HTTP request panicked",
			"event", "http_panic",
			"panic", fmt.Sprint(recovered),
			"stack", string(debug.Stack()),
		)
		writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	})
}

func loggerWithRequest(c *gin.Context, logger *slog.Logger) *slog.Logger {
	logger = normalizeLogger(logger)
	if requestID, ok := c.Get(requestIDContextKey); ok {
		if value, valid := requestID.(string); valid && value != "" {
			return logger.With("request_id", value)
		}
	}
	return logger
}

func normalizeLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

func validRequestID(value string) bool {
	if value == "" || len(value) > maximumRequestIDLen {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' ||
			character == '.' || character == ':' {
			continue
		}
		return false
	}
	return true
}
