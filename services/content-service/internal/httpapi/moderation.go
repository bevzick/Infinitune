package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type moderationReviewRequest struct {
	Decision moderation.Decision `json:"decision"`
	Comment  string              `json:"comment"`
}

type moderationReviewResponse struct {
	ID                   string              `json:"id"`
	ReleaseID            string              `json:"release_id"`
	ReviewerUserID       string              `json:"reviewer_user_id"`
	Decision             moderation.Decision `json:"decision"`
	Comment              string              `json:"comment,omitempty"`
	ReviewedSubmissionAt time.Time           `json:"reviewed_submission_at"`
	ReleaseStatus        release.Status      `json:"release_status"`
	CreatedAt            time.Time           `json:"created_at"`
}

func listReleaseModerationReviews(service ReleaseModeration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, err := queryInteger(c, "limit")
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_moderation_query", "limit must be an integer")
			return
		}
		offset, err := queryInteger(c, "offset")
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_moderation_query", "offset must be an integer")
			return
		}
		verified := requestIdentity(c)
		result, err := service.List(c.Request.Context(), creator.Actor{
			UserID: verified.UserID, Roles: verified.Roles,
		}, moderation.PageRequest{Limit: limit, Offset: offset})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]moderationReviewResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, presentModerationReview(item))
		}
		c.JSON(http.StatusOK, gin.H{
			"items": items,
			"page": releasePageMetadata{
				Limit: result.Limit, Offset: result.Offset, Total: result.Total,
			},
		})
	}
}

func reviewRelease(service ReleaseModeration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, err := uuid.Parse(c.Param("release_id"))
		if err != nil || releaseID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity,
				"invalid_moderation_review", "release_id must be a UUID")
			return
		}
		var request moderationReviewRequest
		if err = decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request_body", err.Error())
			return
		}
		verified := requestIdentity(c)
		result, err := service.Review(c.Request.Context(), creator.Actor{
			UserID: verified.UserID, Roles: verified.Roles,
		}, moderation.Input{
			ReleaseID: releaseID, Decision: request.Decision, Comment: request.Comment,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentModerationReview(result))
	}
}

func presentModerationReview(result moderation.Review) moderationReviewResponse {
	return moderationReviewResponse{
		ID: result.ID.String(), ReleaseID: result.ReleaseID.String(),
		ReviewerUserID: result.ReviewerUserID.String(), Decision: result.Decision,
		Comment: result.Comment, ReviewedSubmissionAt: result.ReviewedSubmissionAt,
		ReleaseStatus: result.ReleaseStatus, CreatedAt: result.CreatedAt,
	}
}
