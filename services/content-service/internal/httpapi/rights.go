package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type setTrackRightsRequest struct {
	TerritoryCode string      `json:"territory_code"`
	Type          rights.Type `json:"rights_type"`
	ValidFrom     string      `json:"valid_from"`
	ValidUntil    *string     `json:"valid_until"`
}

type trackRightsResponse struct {
	ID              string      `json:"id"`
	ReleaseTrackID  string      `json:"release_track_id"`
	TerritoryCode   string      `json:"territory_code"`
	Type            rights.Type `json:"rights_type"`
	ValidFrom       string      `json:"valid_from"`
	ValidUntil      *string     `json:"valid_until"`
	GrantedByUserID string      `json:"granted_by_user_id"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

func setTrackRights(service TrackRights, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		trackID, ok := rightsPathID(c, "release_track_id")
		if !ok {
			return
		}
		var request setTrackRightsRequest
		if decodeJSON(c, &request) != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "Request body is invalid")
			return
		}
		validFrom, err := time.Parse(time.DateOnly, request.ValidFrom)
		if err != nil {
			writeError(c, http.StatusUnprocessableEntity,
				"invalid_track_rights", "valid_from must use YYYY-MM-DD")
			return
		}
		var validUntil *time.Time
		if request.ValidUntil != nil {
			parsed, parseErr := time.Parse(time.DateOnly, *request.ValidUntil)
			if parseErr != nil {
				writeError(c, http.StatusUnprocessableEntity,
					"invalid_track_rights", "valid_until must use YYYY-MM-DD")
				return
			}
			validUntil = &parsed
		}
		stored, err := service.Set(c.Request.Context(), requestIdentity(c).UserID, rights.SetInput{
			ReleaseTrackID: trackID, TerritoryCode: request.TerritoryCode,
			Type: request.Type, ValidFrom: validFrom, ValidUntil: validUntil,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentTrackRights(stored))
	}
}

func listTrackRights(service TrackRights, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		trackID, ok := rightsPathID(c, "release_track_id")
		if !ok {
			return
		}
		values, err := service.List(c.Request.Context(), requestIdentity(c).UserID, trackID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		response := make([]trackRightsResponse, 0, len(values))
		for _, value := range values {
			response = append(response, presentTrackRights(value))
		}
		c.JSON(http.StatusOK, response)
	}
}

func deleteTrackRights(service TrackRights, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		rightID, ok := rightsPathID(c, "right_id")
		if !ok {
			return
		}
		if err := service.Delete(
			c.Request.Context(), requestIdentity(c).UserID, rightID,
		); err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func rightsPathID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil || id == uuid.Nil {
		writeError(c, http.StatusUnprocessableEntity,
			"invalid_track_rights", name+" must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}

func presentTrackRights(value rights.Right) trackRightsResponse {
	var validUntil *string
	if value.ValidUntil != nil {
		formatted := value.ValidUntil.Format(time.DateOnly)
		validUntil = &formatted
	}
	return trackRightsResponse{
		ID: value.ID.String(), ReleaseTrackID: value.ReleaseTrackID.String(),
		TerritoryCode: value.TerritoryCode, Type: value.Type,
		ValidFrom: value.ValidFrom.Format(time.DateOnly), ValidUntil: validUntil,
		GrantedByUserID: value.GrantedByUserID.String(),
		CreatedAt:       value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
