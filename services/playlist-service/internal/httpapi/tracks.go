package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type addTrackRequest struct {
	TrackID uuid.UUID `json:"track_id"`
}

type reorderTracksRequest struct {
	TrackIDs []uuid.UUID `json:"track_ids"`
}

type trackResponse struct {
	TrackID  uuid.UUID `json:"track_id"`
	Position int       `json:"position"`
	AddedBy  uuid.UUID `json:"added_by"`
	AddedAt  time.Time `json:"added_at"`
}

type trackPageResponse struct {
	Items []trackResponse `json:"items"`
	Page  pageResponse    `json:"page"`
}

type pageResponse struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

func getTracks(service PlaylistTracks, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		playlistID, ok := pathUUID(c, "playlist_id")
		if !ok {
			return
		}
		page, ok := trackPageRequest(c)
		if !ok {
			return
		}
		tracks, err := service.Tracks(c.Request.Context(), viewerID(c), playlistID, page)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, trackPageResponse{
			Items: tracksResponse(tracks.Items),
			Page: pageResponse{
				Limit: tracks.Limit, Offset: tracks.Offset, Total: tracks.Total,
			},
		})
	}
}

func trackPageRequest(c *gin.Context) (playlist.PageRequest, bool) {
	limit, ok := queryInteger(c, "limit")
	if !ok {
		return playlist.PageRequest{}, false
	}
	offset, ok := queryInteger(c, "offset")
	if !ok {
		return playlist.PageRequest{}, false
	}
	return playlist.PageRequest{Limit: limit, Offset: offset}, true
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

func addTrack(service PlaylistTracks, logger *slog.Logger) gin.HandlerFunc {
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
		var request addTrackRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		added, err := service.Add(c.Request.Context(), actor.UserID, playlistID, request.TrackID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusCreated, trackResponseFrom(added))
	}
}

func removeTrack(service PlaylistTracks, logger *slog.Logger) gin.HandlerFunc {
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
		trackID, ok := pathUUID(c, "track_id")
		if !ok {
			return
		}
		if err := service.Remove(c.Request.Context(), actor.UserID, playlistID, trackID); err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func reorderTracks(service PlaylistTracks, logger *slog.Logger) gin.HandlerFunc {
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
		var request reorderTracksRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		tracks, err := service.Reorder(c.Request.Context(), actor.UserID, playlistID, request.TrackIDs)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, tracksResponse(tracks))
	}
}

func trackResponseFrom(value playlist.Track) trackResponse {
	return trackResponse{
		TrackID: value.TrackID, Position: value.Position,
		AddedBy: value.AddedBy, AddedAt: value.AddedAt,
	}
}

func tracksResponse(values []playlist.Track) []trackResponse {
	result := make([]trackResponse, 0, len(values))
	for _, value := range values {
		result = append(result, trackResponseFrom(value))
	}
	return result
}
