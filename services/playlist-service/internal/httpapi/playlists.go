package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createPlaylistRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Visibility  playlist.Visibility `json:"visibility"`
}

type updatePlaylistRequest struct {
	Name        *string              `json:"name"`
	Description *string              `json:"description"`
	Visibility  *playlist.Visibility `json:"visibility"`
}

type playlistResponse struct {
	ID          uuid.UUID           `json:"id"`
	OwnerID     uuid.UUID           `json:"owner_id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        playlist.Type       `json:"type"`
	Visibility  playlist.Visibility `json:"visibility"`
	RuleSetID   *uuid.UUID          `json:"rule_set_id,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type playlistPageResponse struct {
	Items []playlistResponse `json:"items"`
	Page  pageResponse       `json:"page"`
}

func listPlaylists(service Playlists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
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
		result, err := service.List(c.Request.Context(), actor.UserID, playlist.PageRequest{
			Limit: limit, Offset: offset,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]playlistResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, playlistResponseFrom(item))
		}
		c.JSON(http.StatusOK, playlistPageResponse{
			Items: items,
			Page: pageResponse{
				Limit: result.Limit, Offset: result.Offset, Total: result.Total,
			},
		})
	}
}

func createPlaylist(service Playlists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		var request createPlaylistRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		created, err := service.Create(c.Request.Context(), actor.UserID, playlist.CreateInput{
			Name: request.Name, Description: request.Description, Visibility: request.Visibility,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Location", "/v1/playlists/"+created.ID.String())
		c.JSON(http.StatusCreated, playlistResponseFrom(created))
	}
}

func getPlaylist(service Playlists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		playlistID, ok := pathUUID(c, "playlist_id")
		if !ok {
			return
		}
		result, err := service.ByID(c.Request.Context(), viewerID(c), playlistID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, playlistResponseFrom(result))
	}
}

func updatePlaylist(service Playlists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		playlistID, ok := pathUUID(c, "playlist_id")
		if !ok {
			return
		}
		var request updatePlaylistRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		updated, err := service.Update(c.Request.Context(), actor.UserID, playlistID, playlist.UpdateInput{
			Name: request.Name, Description: request.Description, Visibility: request.Visibility,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, playlistResponseFrom(updated))
	}
}

func deletePlaylist(service Playlists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		playlistID, ok := pathUUID(c, "playlist_id")
		if !ok {
			return
		}
		if err := service.Delete(c.Request.Context(), actor.UserID, playlistID); err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func playlistResponseFrom(value playlist.Playlist) playlistResponse {
	return playlistResponse{
		ID: value.ID, OwnerID: value.OwnerID, Name: value.Name, Description: value.Description,
		Type: value.Type, Visibility: value.Visibility, RuleSetID: value.RuleSetID,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func pathUUID(c *gin.Context, parameter string) (uuid.UUID, bool) {
	value, err := uuid.Parse(c.Param(parameter))
	if err != nil || value == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_"+parameter,
			parameter+" must be a UUID")
		return uuid.Nil, false
	}
	return value, true
}
