package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createCommentRequest struct {
	TrackID  uuid.UUID  `json:"track_id"`
	ParentID *uuid.UUID `json:"parent_id"`
	Content  string     `json:"content"`
}
type updateCommentRequest struct {
	Content string `json:"content"`
}
type commentResponse struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TrackID   uuid.UUID  `json:"track_id"`
	ParentID  *uuid.UUID `json:"parent_id"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
type pageResponse struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}
type commentPageResponse struct {
	Items []commentResponse `json:"items"`
	Page  pageResponse      `json:"page"`
}

func createComment(service Comments, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		var request createCommentRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		created, err := service.Create(c.Request.Context(), actor.UserID, comment.CreateInput{
			TrackID: request.TrackID, ParentID: request.ParentID, Content: request.Content,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Location", "/v1/comments/"+created.ID.String())
		c.JSON(http.StatusCreated, commentResponseFrom(created))
	}
}

func listComments(service Comments, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		trackID, ok := pathUUID(c, "track_id")
		if !ok {
			return
		}
		page, ok := pageRequest(c)
		if !ok {
			return
		}
		result, err := service.List(c.Request.Context(), trackID, page)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]commentResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, commentResponseFrom(item))
		}
		c.JSON(http.StatusOK, commentPageResponse{Items: items, Page: pageResponse{
			Limit: result.Limit, Offset: result.Offset, Total: result.Total,
		}})
	}
}

func updateComment(service Comments, logger *slog.Logger) gin.HandlerFunc {
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
		var request updateCommentRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		updated, err := service.Update(c.Request.Context(), actor.UserID, commentID, request.Content)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, commentResponseFrom(updated))
	}
}

func deleteComment(service Comments, logger *slog.Logger) gin.HandlerFunc {
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
		if err := service.Delete(c.Request.Context(), actor.UserID, commentID); err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func commentResponseFrom(value comment.Comment) commentResponse {
	return commentResponse{ID: value.ID, UserID: value.UserID, TrackID: value.TrackID,
		ParentID: value.ParentID, Content: value.Content, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func pathUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	value, err := uuid.Parse(c.Param(key))
	if err != nil || value == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_"+key, key+" must be a UUID")
		return uuid.Nil, false
	}
	return value, true
}
func pageRequest(c *gin.Context) (comment.PageRequest, bool) {
	limit, ok := queryInteger(c, "limit")
	if !ok {
		return comment.PageRequest{}, false
	}
	offset, ok := queryInteger(c, "offset")
	if !ok {
		return comment.PageRequest{}, false
	}
	return comment.PageRequest{Limit: limit, Offset: offset}, true
}
func queryInteger(c *gin.Context, key string) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_"+key, key+" must be an integer")
		return 0, false
	}
	return value, true
}
