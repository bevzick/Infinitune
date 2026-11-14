package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type followResponse struct {
	ArtistID   uuid.UUID `json:"artist_id"`
	FollowedAt time.Time `json:"followed_at"`
}

type followPageResponse struct {
	Items []followResponse `json:"items"`
	Page  pageResponse     `json:"page"`
}

func followArtist(service Follows, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		artistID, ok := pathUUID(c, "artist_id")
		if !ok {
			return
		}
		followed, err := service.FollowArtist(c.Request.Context(), actor.UserID, artistID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, followResponse{
			ArtistID: followed.ArtistID, FollowedAt: followed.FollowedAt,
		})
	}
}

func unfollowArtist(service Follows, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		artistID, ok := pathUUID(c, "artist_id")
		if !ok {
			return
		}
		if err := service.UnfollowArtist(c.Request.Context(), actor.UserID, artistID); err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func listArtistFollows(service Follows, logger *slog.Logger) gin.HandlerFunc {
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
		result, err := service.ArtistFollows(c.Request.Context(), actor.UserID, page)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]followResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, followResponse{
				ArtistID: item.ArtistID, FollowedAt: item.FollowedAt,
			})
		}
		c.JSON(http.StatusOK, followPageResponse{Items: items, Page: pageResponse{
			Limit: result.Limit, Offset: result.Offset, Total: result.Total,
		}})
	}
}
