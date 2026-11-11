package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type trackLyricsResponse struct {
	ID              uuid.UUID                 `json:"id"`
	TrackID         uuid.UUID                 `json:"track_id"`
	Type            metadata.LyricsType       `json:"lyrics_type"`
	Language        string                    `json:"language"`
	Content         string                    `json:"content"`
	SourceType      metadata.LyricsSourceType `json:"source_type"`
	SourceName      string                    `json:"source_name"`
	CopyrightNotice string                    `json:"copyright_notice"`
	Official        bool                      `json:"official"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

func trackLyricsHandler(
	logger *slog.Logger,
	fetch func(context.Context, uuid.UUID) ([]metadata.TrackLyrics, error),
) gin.HandlerFunc {
	return func(c *gin.Context) {
		trackID, err := uuid.Parse(c.Param("id"))
		if err != nil || trackID == uuid.Nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "must be a UUID", "id")
			return
		}
		lyrics, err := fetch(c.Request.Context(), trackID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		response := make([]trackLyricsResponse, 0, len(lyrics))
		for _, value := range lyrics {
			response = append(response, trackLyricsResponse{
				ID: value.ID, TrackID: value.TrackID, Type: value.Type,
				Language: value.Language, Content: value.Content,
				SourceType: value.SourceType, SourceName: value.SourceName,
				CopyrightNotice: value.CopyrightNotice, Official: value.Official,
				UpdatedAt: value.UpdatedAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": response})
	}
}
