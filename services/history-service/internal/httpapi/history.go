package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type eventResponse struct {
	EventID     uuid.UUID  `json:"event_id"`
	TrackID     uuid.UUID  `json:"track_id"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	ListenedMS  int        `json:"listened_ms"`
	Completed   bool       `json:"completed"`
	ContextType string     `json:"context_type"`
	ContextID   *uuid.UUID `json:"context_id"`
	DeviceType  string     `json:"device_type"`
}
type historyPageResponse struct {
	Items      []eventResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}
type clearHistoryResponse struct {
	Deleted int64 `json:"deleted"`
}
type cursorDocument struct {
	StartedAt time.Time `json:"started_at"`
	EventID   uuid.UUID `json:"event_id"`
}

func listEvents(service History, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		request, err := listRequest(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_history_query", err.Error())
			return
		}
		page, err := service.List(c.Request.Context(), actor.UserID, request)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]eventResponse, 0, len(page.Items))
		for _, event := range page.Items {
			items = append(items, eventResponseFrom(event))
		}
		response := historyPageResponse{Items: items}
		if page.NextCursor != nil {
			response.NextCursor = encodeCursor(*page.NextCursor)
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, response)
	}
}

func clearEvents(service History, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		if err := validateQueryKeys(c, map[string]struct{}{"before": {}}); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_history_query", err.Error())
			return
		}
		before, err := optionalTime(c, "before")
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_history_query", err.Error())
			return
		}
		result, err := service.Clear(c.Request.Context(), actor.UserID, history.ClearRequest{Before: before})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, clearHistoryResponse{Deleted: result.Deleted})
	}
}

func listRequest(c *gin.Context) (history.ListRequest, error) {
	allowed := map[string]struct{}{"track_id": {}, "from": {}, "to": {}, "completed": {}, "context_type": {}, "device_type": {}, "limit": {}, "cursor": {}}
	if err := validateQueryKeys(c, allowed); err != nil {
		return history.ListRequest{}, err
	}
	trackID, err := optionalUUID(c, "track_id")
	if err != nil {
		return history.ListRequest{}, err
	}
	from, err := optionalTime(c, "from")
	if err != nil {
		return history.ListRequest{}, err
	}
	to, err := optionalTime(c, "to")
	if err != nil {
		return history.ListRequest{}, err
	}
	completed, err := optionalBool(c, "completed")
	if err != nil {
		return history.ListRequest{}, err
	}
	limit, err := optionalInt(c, "limit")
	if err != nil {
		return history.ListRequest{}, err
	}
	cursor, err := decodeCursor(c.Query("cursor"))
	if err != nil {
		return history.ListRequest{}, err
	}
	return history.ListRequest{Limit: limit, Filter: history.ListFilter{TrackID: trackID, From: from, To: to, Completed: completed, ContextType: c.Query("context_type"), DeviceType: c.Query("device_type"), Cursor: cursor}}, nil
}

func validateQueryKeys(c *gin.Context, allowed map[string]struct{}) error {
	for key, values := range c.Request.URL.Query() {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unsupported query parameter %q", key)
		}
		if len(values) != 1 {
			return fmt.Errorf("query parameter %q must be provided once", key)
		}
	}
	return nil
}
func optionalUUID(c *gin.Context, key string) (*uuid.UUID, error) {
	value := c.Query(key)
	if value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		return nil, fmt.Errorf("%s must be a UUID", key)
	}
	return &parsed, nil
}
func optionalTime(c *gin.Context, key string) (*time.Time, error) {
	value := c.Query(key)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp", key)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
func optionalBool(c *gin.Context, key string) (*bool, error) {
	value := c.Query(key)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be true or false", key)
	}
	return &parsed, nil
}
func optionalInt(c *gin.Context, key string) (int, error) {
	value := c.Query(key)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}
func encodeCursor(cursor history.Cursor) string {
	document, _ := json.Marshal(cursorDocument{StartedAt: cursor.StartedAt.UTC(), EventID: cursor.EventID})
	return base64.RawURLEncoding.EncodeToString(document)
}
func decodeCursor(value string) (*history.Cursor, error) {
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("cursor is invalid")
	}
	var document cursorDocument
	if json.Unmarshal(decoded, &document) != nil || document.StartedAt.IsZero() || document.EventID == uuid.Nil {
		return nil, fmt.Errorf("cursor is invalid")
	}
	return &history.Cursor{StartedAt: document.StartedAt.UTC(), EventID: document.EventID}, nil
}
func eventResponseFrom(event history.Event) eventResponse {
	return eventResponse{EventID: event.ID, TrackID: event.TrackID, StartedAt: event.StartedAt, FinishedAt: event.FinishedAt, ListenedMS: event.ListenedMS, Completed: event.Completed, ContextType: event.ContextType, ContextID: event.ContextID, DeviceType: event.DeviceType}
}
