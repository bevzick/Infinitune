package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type recordEventRequest struct {
	EventID     uuid.UUID  `json:"event_id"`
	TrackID     uuid.UUID  `json:"track_id"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  time.Time  `json:"finished_at"`
	ListenedMS  int        `json:"listened_ms"`
	ContextType string     `json:"context_type"`
	ContextID   *uuid.UUID `json:"context_id"`
	DeviceType  string     `json:"device_type"`
}
type recordEventResponse struct {
	EventID   uuid.UUID `json:"event_id"`
	Recorded  bool      `json:"recorded"`
	Completed bool      `json:"completed"`
}

func recordEvent(service History, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		var request recordEventRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		result, err := service.Record(c.Request.Context(), actor.UserID, history.RecordInput{
			EventID: request.EventID, TrackID: request.TrackID, StartedAt: request.StartedAt, FinishedAt: request.FinishedAt,
			ListenedMS: request.ListenedMS, ContextType: request.ContextType, ContextID: request.ContextID, DeviceType: request.DeviceType,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, recordEventResponse{EventID: result.EventID, Recorded: result.Recorded, Completed: result.Completed})
	}
}
