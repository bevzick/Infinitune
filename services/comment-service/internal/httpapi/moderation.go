package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/bevzick/Infinitune/services/comment-service/internal/moderation"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type moderationActionRequest struct {
	Action moderation.Action `json:"action"`
	Note   string            `json:"note"`
}

type moderationDecisionResponse struct {
	ID                     uuid.UUID            `json:"id"`
	ReportID               uuid.UUID            `json:"report_id"`
	ModeratorUserID        uuid.UUID            `json:"moderator_user_id"`
	Action                 moderation.Action    `json:"action"`
	Note                   string               `json:"note"`
	PreviousReportStatus   comment.ReportStatus `json:"previous_report_status"`
	ResultingReportStatus  comment.ReportStatus `json:"resulting_report_status"`
	PreviousCommentStatus  comment.Status       `json:"previous_comment_status"`
	ResultingCommentStatus comment.Status       `json:"resulting_comment_status"`
	CreatedAt              time.Time            `json:"created_at"`
}

type moderationCaseResponse struct {
	Report  moderationReportResponse  `json:"report"`
	Comment moderationCommentResponse `json:"comment"`
}
type moderationReportResponse struct {
	ID             uuid.UUID            `json:"id"`
	CommentID      uuid.UUID            `json:"comment_id"`
	ReporterUserID uuid.UUID            `json:"reporter_user_id"`
	Reason         comment.ReportReason `json:"reason"`
	Description    string               `json:"description"`
	Status         comment.ReportStatus `json:"status"`
	CreatedAt      time.Time            `json:"created_at"`
}
type moderationCommentResponse struct {
	ID       uuid.UUID      `json:"id"`
	UserID   uuid.UUID      `json:"user_id"`
	TrackID  uuid.UUID      `json:"track_id"`
	ParentID *uuid.UUID     `json:"parent_id"`
	Content  string         `json:"content"`
	Status   comment.Status `json:"status"`
}

func listModerationActions(service CommentModeration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := moderationActor(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		limit, ok := queryInteger(c, "limit")
		if !ok {
			return
		}
		offset, ok := queryInteger(c, "offset")
		if !ok {
			return
		}
		result, err := service.ListActions(c.Request.Context(), actor,
			moderation.ActionPageRequest{Limit: limit, Offset: offset})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]moderationDecisionResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, presentModerationDecision(item))
		}
		c.JSON(http.StatusOK, gin.H{
			"items": items,
			"page": pageResponse{
				Limit: result.Limit, Offset: result.Offset, Total: result.Total,
			},
		})
	}
}

func listModerationCases(service CommentModeration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := moderationActor(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		page, ok := moderationPageRequest(c)
		if !ok {
			return
		}
		result, err := service.List(c.Request.Context(), actor, page)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]moderationCaseResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, moderationCaseResponse{
				Report: moderationReportResponse{
					ID: item.Report.ID, CommentID: item.Report.CommentID,
					ReporterUserID: item.Report.ReporterUserID, Reason: item.Report.Reason,
					Description: item.Report.Description, Status: item.Report.Status,
					CreatedAt: item.Report.CreatedAt,
				},
				Comment: moderationCommentResponse{
					ID: item.Comment.ID, UserID: item.Comment.UserID, TrackID: item.Comment.TrackID,
					ParentID: item.Comment.ParentID, Content: item.Comment.Content, Status: item.Comment.Status,
				},
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"items": items,
			"page":  pageResponse{Limit: result.Limit, Offset: result.Offset, Total: result.Total},
		})
	}
}

func applyModerationAction(service CommentModeration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := moderationActor(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		reportID, ok := pathUUID(c, "report_id")
		if !ok {
			return
		}
		var request moderationActionRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		result, err := service.Apply(c.Request.Context(), actor, moderation.Input{
			ReportID: reportID, Action: request.Action, Note: request.Note,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentModerationDecision(result))
	}
}

func presentModerationDecision(result moderation.Decision) moderationDecisionResponse {
	return moderationDecisionResponse{
		ID: result.ID, ReportID: result.ReportID, ModeratorUserID: result.ModeratorUserID,
		Action: result.Action, Note: result.Note,
		PreviousReportStatus:   result.PreviousReportStatus,
		ResultingReportStatus:  result.ResultingReportStatus,
		PreviousCommentStatus:  result.PreviousCommentStatus,
		ResultingCommentStatus: result.ResultingCommentStatus,
		CreatedAt:              result.CreatedAt,
	}
}

func moderationActor(c *gin.Context) (moderation.Actor, bool) {
	verified, ok := requestIdentity(c)
	return moderation.Actor{UserID: verified.UserID, Roles: verified.Roles}, ok
}

func moderationPageRequest(c *gin.Context) (moderation.PageRequest, bool) {
	limit, ok := queryInteger(c, "limit")
	if !ok {
		return moderation.PageRequest{}, false
	}
	offset, ok := queryInteger(c, "offset")
	if !ok {
		return moderation.PageRequest{}, false
	}
	var status *comment.ReportStatus
	if raw := c.Query("status"); raw != "" {
		value := comment.ReportStatus(raw)
		status = &value
	}
	return moderation.PageRequest{Status: status, Limit: limit, Offset: offset}, true
}
