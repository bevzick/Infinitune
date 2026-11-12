package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createUploadSessionRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	OriginalName   string `json:"original_name"`
	MIMEType       string `json:"mime_type"`
	SizeBytes      int64  `json:"size_bytes"`
}

type uploadSessionResponse struct {
	ID             string        `json:"id"`
	ReleaseTrackID string        `json:"release_track_id"`
	UploadURL      string        `json:"upload_url"`
	OriginalName   string        `json:"original_name"`
	MIMEType       string        `json:"mime_type"`
	SizeBytes      int64         `json:"size_bytes"`
	Status         upload.Status `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func createUploadSession(service UploadSessions, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		trackID, err := uuid.Parse(c.Param("release_track_id"))
		if err != nil || trackID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity,
				"invalid_upload_session", "release_track_id must be a UUID")
			return
		}
		var request createUploadSessionRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		idempotencyKey, err := uuid.Parse(request.IdempotencyKey)
		if err != nil || idempotencyKey == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity,
				"invalid_upload_session", "idempotency_key must be a UUID")
			return
		}
		session, err := service.CreateSession(c.Request.Context(), requestIdentity(c).UserID,
			upload.CreateInput{
				ReleaseTrackID: trackID, IdempotencyKey: idempotencyKey,
				OriginalName: request.OriginalName, MIMEType: request.MIMEType,
				SizeBytes: request.SizeBytes,
			})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Location", "/v1/media/uploads/"+session.ID.String())
		c.JSON(http.StatusCreated, presentUploadSession(session))
	}
}

func presentUploadSession(session upload.Session) uploadSessionResponse {
	return uploadSessionResponse{
		ID: session.ID.String(), ReleaseTrackID: session.ReleaseTrackID.String(),
		UploadURL:    "/v1/media/uploads/" + session.ID.String(),
		OriginalName: session.OriginalName, MIMEType: session.MIMEType,
		SizeBytes: session.SizeBytes, Status: session.Status,
		CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
	}
}
