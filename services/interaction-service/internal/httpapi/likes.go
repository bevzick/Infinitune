package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type likeResponse struct {
	ID      uuid.UUID `json:"id"`
	LikedAt time.Time `json:"liked_at"`
}

type pageResponse struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

type likePageResponse struct {
	Items []likeResponse `json:"items"`
	Page  pageResponse   `json:"page"`
}

func likeTrack(service Likes, logger *slog.Logger) gin.HandlerFunc {
	return likeTarget("track_id", service.LikeTrack, logger)
}

func unlikeTrack(service Likes, logger *slog.Logger) gin.HandlerFunc {
	return unlikeTarget("track_id", service.UnlikeTrack, logger)
}

func listTrackLikes(service Likes, logger *slog.Logger) gin.HandlerFunc {
	return listLikes(service.TrackLikes, logger)
}

func likeAlbum(service Likes, logger *slog.Logger) gin.HandlerFunc {
	return likeTarget("album_id", service.LikeAlbum, logger)
}

func unlikeAlbum(service Likes, logger *slog.Logger) gin.HandlerFunc {
	return unlikeTarget("album_id", service.UnlikeAlbum, logger)
}

func listAlbumLikes(service Likes, logger *slog.Logger) gin.HandlerFunc {
	return listLikes(service.AlbumLikes, logger)
}

func likeTarget(
	pathKey string,
	like func(context.Context, uuid.UUID, uuid.UUID) (interaction.Like, error),
	logger *slog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		targetID, ok := pathUUID(c, pathKey)
		if !ok {
			return
		}
		liked, err := like(c.Request.Context(), actor.UserID, targetID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, likeResponse{ID: liked.TargetID, LikedAt: liked.CreatedAt})
	}
}

func unlikeTarget(
	pathKey string,
	unlike func(context.Context, uuid.UUID, uuid.UUID) error,
	logger *slog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		targetID, ok := pathUUID(c, pathKey)
		if !ok {
			return
		}
		if err := unlike(c.Request.Context(), actor.UserID, targetID); err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func listLikes(
	list func(context.Context, uuid.UUID, interaction.PageRequest) (interaction.LikePage, error),
	logger *slog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		page, ok := pageRequest(c)
		if !ok {
			return
		}
		result, err := list(c.Request.Context(), actor.UserID, page)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]likeResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, likeResponse{ID: item.TargetID, LikedAt: item.CreatedAt})
		}
		c.JSON(http.StatusOK, likePageResponse{Items: items, Page: pageResponse{
			Limit: result.Limit, Offset: result.Offset, Total: result.Total,
		}})
	}
}

func pathUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	value, err := uuid.Parse(c.Param(key))
	if err != nil || value == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_"+key, key+" must be a UUID")
		return uuid.Nil, false
	}
	return value, true
}

func pageRequest(c *gin.Context) (interaction.PageRequest, bool) {
	limit, ok := queryInteger(c, "limit")
	if !ok {
		return interaction.PageRequest{}, false
	}
	offset, ok := queryInteger(c, "offset")
	if !ok {
		return interaction.PageRequest{}, false
	}
	return interaction.PageRequest{Limit: limit, Offset: offset}, true
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
