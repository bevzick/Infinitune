package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
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
	case errors.Is(err, comment.ErrInvalidArgument):
		writeError(c, http.StatusUnprocessableEntity, "invalid_comment", err.Error())
	case errors.Is(err, comment.ErrNotFound):
		writeError(c, http.StatusNotFound, "comment_not_found", "Comment was not found")
	case errors.Is(err, comment.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Operation is forbidden")
	case errors.Is(err, comment.ErrNotEditable):
		writeError(c, http.StatusConflict, "comment_not_editable", "Comment is not editable")
	case errors.Is(err, comment.ErrParentInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_parent", "Parent comment is invalid")
	case errors.Is(err, comment.ErrRateLimited):
		c.Header("Retry-After", "60")
		writeError(c, http.StatusTooManyRequests, "comment_rate_limited", "Too many comments; retry later")
	case errors.Is(err, comment.ErrReportDuplicate):
		writeError(c, http.StatusConflict, "comment_already_reported", "Comment was already reported")
	case errors.Is(err, comment.ErrReportRateLimited):
		c.Header("Retry-After", "3600")
		writeError(c, http.StatusTooManyRequests, "report_rate_limited", "Too many reports; retry later")
	case errors.Is(err, comment.ErrOwnCommentReport):
		writeError(c, http.StatusUnprocessableEntity, "own_comment_report", "Own comment cannot be reported")
	case errors.Is(err, comment.ErrTrackNotFound):
		writeError(c, http.StatusNotFound, "track_not_found", "Track was not found")
	case errors.Is(err, comment.ErrCatalogTimeout):
		writeError(c, http.StatusGatewayTimeout, "catalog_timeout", "Catalog service did not respond in time")
	case errors.Is(err, comment.ErrCatalogUnavailable):
		writeError(c, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog service is temporarily unavailable")
	default:
		logger.Error("comment request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed")
	}
}
