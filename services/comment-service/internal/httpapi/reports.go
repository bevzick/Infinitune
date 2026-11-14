package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type reportCommentRequest struct {
	Reason      comment.ReportReason `json:"reason"`
	Description string               `json:"description"`
}

type reportResponse struct {
	ID          uuid.UUID            `json:"id"`
	CommentID   uuid.UUID            `json:"comment_id"`
	Reason      comment.ReportReason `json:"reason"`
	Description string               `json:"description"`
	Status      comment.ReportStatus `json:"status"`
	CreatedAt   time.Time            `json:"created_at"`
}

func reportComment(service Comments, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		commentID, ok := pathUUID(c, "comment_id")
		if !ok {
			return
		}
		var request reportCommentRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		created, err := service.Report(c.Request.Context(), actor.UserID, commentID, comment.ReportInput{
			Reason: request.Reason, Description: request.Description,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Location", "/v1/comments/"+commentID.String()+"/reports/"+created.ID.String())
		c.JSON(http.StatusCreated, reportResponse{
			ID: created.ID, CommentID: created.CommentID, Reason: created.Reason,
			Description: created.Description, Status: created.Status, CreatedAt: created.CreatedAt,
		})
	}
}
