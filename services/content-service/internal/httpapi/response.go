package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/publication"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/bevzick/Infinitune/services/content-service/internal/submission"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/gin-gonic/gin"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: errorBody{Code: code, Message: message}})
}
func decodeJSON(c *gin.Context, destination any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}
func writeDomainError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, upload.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_upload_session", err.Error())
	case errors.Is(err, upload.ErrNotFound):
		writeError(c, http.StatusNotFound, "release_track_not_found", "Release track was not found")
	case errors.Is(err, upload.ErrIdempotencyConflict):
		writeError(c, http.StatusConflict, "upload_idempotency_conflict", err.Error())
	case errors.Is(err, upload.ErrReleaseLocked):
		writeError(c, http.StatusConflict, "release_upload_locked", "Release does not accept uploads")
	case errors.Is(err, upload.ErrConflict):
		writeError(c, http.StatusConflict, "upload_session_conflict", "Upload session changed concurrently")
	case errors.Is(err, release.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_release", err.Error())
	case errors.Is(err, release.ErrNotFound):
		writeError(c, http.StatusNotFound, "release_not_found", "Release was not found")
	case errors.Is(err, release.ErrConflict):
		writeError(c, http.StatusConflict, "release_lifecycle_conflict", "Release cannot be changed in its current status")
	case errors.Is(err, release.ErrCatalogUnavailable):
		writeError(c, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog archival is temporarily unavailable")
	case errors.Is(err, publication.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_release_publication", err.Error())
	case errors.Is(err, publication.ErrNotFound):
		writeError(c, http.StatusNotFound, "release_not_found", "Release was not found")
	case errors.Is(err, publication.ErrNotReady):
		writeError(c, http.StatusConflict, "release_not_ready", err.Error())
	case errors.Is(err, publication.ErrConflict):
		writeError(c, http.StatusConflict, "release_publication_conflict", "Release cannot be published in its current status")
	case errors.Is(err, publication.ErrMediaUnavailable):
		writeError(c, http.StatusServiceUnavailable, "media_unavailable", "Release media could not be loaded")
	case errors.Is(err, publication.ErrCatalogUnavailable):
		writeError(c, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog publication is temporarily unavailable")
	case errors.Is(err, publication.ErrInconsistentMedia):
		logger.Error("release media association is inconsistent", "error", err)
		writeError(c, http.StatusInternalServerError, "inconsistent_media", "Release media association is inconsistent")
	case errors.Is(err, publication.ErrInconsistentCatalog):
		logger.Error("catalog publication response is inconsistent", "error", err)
		writeError(c, http.StatusInternalServerError, "inconsistent_catalog", "Catalog publication response is inconsistent")
	case errors.Is(err, moderation.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_moderation_review", err.Error())
	case errors.Is(err, moderation.ErrNotFound):
		writeError(c, http.StatusNotFound, "release_not_found", "Release was not found")
	case errors.Is(err, moderation.ErrForbidden):
		writeError(c, http.StatusForbidden, "moderator_role_required", "Moderator role is required")
	case errors.Is(err, moderation.ErrSelfReview):
		writeError(c, http.StatusForbidden, "self_review_forbidden", "Creator members cannot review their own release")
	case errors.Is(err, moderation.ErrConflict):
		writeError(c, http.StatusConflict, "moderation_review_conflict", "Release submission is already reviewed or unavailable")
	case errors.Is(err, creator.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_creator", err.Error())
	case errors.Is(err, creator.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Operation is forbidden")
	case errors.Is(err, creator.ErrVerificationRequired):
		writeError(c, http.StatusForbidden, "verification_required", "Verified creator account is required")
	case errors.Is(err, rights.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_track_rights", err.Error())
	case errors.Is(err, rights.ErrNotFound):
		writeError(c, http.StatusNotFound, "track_rights_not_found", "Track or track rights were not found")
	case errors.Is(err, rights.ErrImmutable):
		writeError(c, http.StatusConflict, "track_rights_immutable", "Track rights cannot be changed in the current release status")
	case errors.Is(err, submission.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_release_submission", err.Error())
	case errors.Is(err, submission.ErrNotFound):
		writeError(c, http.StatusNotFound, "release_not_found", "Release was not found")
	case errors.Is(err, submission.ErrNotReady):
		writeError(c, http.StatusConflict, "release_not_ready", err.Error())
	case errors.Is(err, submission.ErrConflict):
		writeError(c, http.StatusConflict, "release_submission_conflict", "Release cannot be submitted in its current status")
	case errors.Is(err, submission.ErrMediaUnavailable):
		writeError(c, http.StatusServiceUnavailable, "media_unavailable", "Media readiness could not be checked")
	case errors.Is(err, submission.ErrInconsistentMedia):
		logger.Error("release media association is inconsistent", "error", err)
		writeError(c, http.StatusInternalServerError, "inconsistent_media", "Release media association is inconsistent")
	case errors.Is(err, creator.ErrTargetUserNotFound):
		writeError(c, http.StatusNotFound, "user_not_found", "User was not found")
	case errors.Is(err, creator.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "Requested resource was not found")
	case errors.Is(err, creator.ErrMemberExists):
		writeError(c, http.StatusConflict, "member_exists", "Creator member already exists")
	case errors.Is(err, creator.ErrOwnerProtected):
		writeError(c, http.StatusConflict, "owner_protected", "Creator owner cannot be changed")
	case errors.Is(err, creator.ErrInvalidTransition):
		writeError(c, http.StatusConflict, "invalid_verification_transition", "Verification status cannot be changed")
	case errors.Is(err, creator.ErrConflict):
		writeError(c, http.StatusConflict, "concurrent_change", "Resource changed concurrently")
	case errors.Is(err, creator.ErrUserDirectoryUnavailable):
		writeError(c, http.StatusServiceUnavailable, "user_service_unavailable", "User service is unavailable")
	default:
		logger.Error("content request failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Request could not be completed")
	}
}
