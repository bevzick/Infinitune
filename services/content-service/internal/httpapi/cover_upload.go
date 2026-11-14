package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/coverupload"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createCoverUploadRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	OriginalName   string `json:"original_name"`
	MIMEType       string `json:"mime_type"`
	SizeBytes      int64  `json:"size_bytes"`
}

type coverUploadResponse struct {
	ID           string             `json:"id"`
	ReleaseID    string             `json:"release_id"`
	UploadURL    string             `json:"upload_url"`
	OriginalName string             `json:"original_name"`
	MIMEType     string             `json:"mime_type"`
	SizeBytes    int64              `json:"size_bytes"`
	Status       coverupload.Status `json:"status"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

func createCoverUploadSession(service CoverUploadSessions, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, err := uuid.Parse(c.Param("release_id"))
		if err != nil || releaseID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_cover_upload", "release_id must be a UUID")
			return
		}
		var request createCoverUploadRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		key, err := uuid.Parse(request.IdempotencyKey)
		if err != nil || key == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_cover_upload", "idempotency_key must be a UUID")
			return
		}
		session, err := service.CreateSession(c.Request.Context(), requestIdentity(c).UserID,
			coverupload.CreateInput{
				ReleaseID: releaseID, IdempotencyKey: key,
				OriginalName: request.OriginalName, MIMEType: request.MIMEType,
				SizeBytes: request.SizeBytes,
			})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Location", "/v1/media/cover-uploads/"+session.ID.String())
		c.JSON(http.StatusCreated, coverUploadResponse{
			ID: session.ID.String(), ReleaseID: session.ReleaseID.String(),
			UploadURL:    "/v1/media/cover-uploads/" + session.ID.String(),
			OriginalName: session.OriginalName, MIMEType: session.MIMEType,
			SizeBytes: session.SizeBytes, Status: session.Status,
			CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
		})
	}
}
