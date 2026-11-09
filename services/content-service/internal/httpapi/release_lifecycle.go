package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const releaseDateLayout = "2006-01-02"

type createReleaseRequest struct {
	CreatorID   string               `json:"creator_id"`
	Title       string               `json:"title"`
	ReleaseType release.Type         `json:"release_type"`
	ReleaseDate string               `json:"release_date"`
	Tracks      []createTrackRequest `json:"tracks"`
}

type createTrackRequest struct {
	Title       string `json:"title"`
	TrackNumber int32  `json:"track_number"`
	DiscNumber  int32  `json:"disc_number"`
	Explicit    bool   `json:"explicit"`
}

type updateReleaseRequest struct {
	Title          string               `json:"title"`
	ReleaseType    release.Type         `json:"release_type"`
	ReleaseDate    string               `json:"release_date"`
	CoverObjectKey *string              `json:"cover_object_key"`
	Tracks         []updateTrackRequest `json:"tracks"`
}

type updateTrackRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	TrackNumber int32  `json:"track_number"`
	DiscNumber  int32  `json:"disc_number"`
	Explicit    bool   `json:"explicit"`
}

type releaseResponse struct {
	ID             string                 `json:"id"`
	CreatorID      string                 `json:"creator_id"`
	Title          string                 `json:"title"`
	ReleaseType    release.Type           `json:"release_type"`
	ReleaseDate    string                 `json:"release_date"`
	Status         release.Status         `json:"status"`
	CoverObjectKey *string                `json:"cover_object_key"`
	CatalogAlbumID *string                `json:"catalog_album_id,omitempty"`
	SubmittedAt    *time.Time             `json:"submitted_at"`
	ReviewedAt     *time.Time             `json:"reviewed_at"`
	PublishedAt    *time.Time             `json:"published_at"`
	ArchivedAt     *time.Time             `json:"archived_at"`
	Tracks         []releaseTrackResponse `json:"tracks"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type releaseTrackResponse struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	TrackNumber    int32   `json:"track_number"`
	DiscNumber     int32   `json:"disc_number"`
	Explicit       bool    `json:"explicit"`
	CatalogTrackID *string `json:"catalog_track_id,omitempty"`
}

func createRelease(service Releases, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request createReleaseRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		creatorID, err := uuid.Parse(request.CreatorID)
		if err != nil || creatorID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_release", "creator_id must be a UUID")
			return
		}
		releaseDate, err := time.Parse(releaseDateLayout, request.ReleaseDate)
		if err != nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_release", "release_date must use YYYY-MM-DD")
			return
		}
		tracks := make([]release.TrackInput, 0, len(request.Tracks))
		for _, track := range request.Tracks {
			tracks = append(tracks, release.TrackInput{
				Title: track.Title, TrackNumber: track.TrackNumber,
				DiscNumber: track.DiscNumber, Explicit: track.Explicit,
			})
		}
		result, err := service.CreateDraft(c.Request.Context(), requestIdentity(c).UserID,
			release.CreateDraftInput{
				CreatorID: creatorID, Title: request.Title, Type: request.ReleaseType,
				ReleaseDate: releaseDate, Tracks: tracks,
			})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusCreated, releaseResponseFrom(result))
	}
}

func getRelease(service Releases, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, ok := releaseIDParam(c)
		if !ok {
			return
		}
		result, err := service.ByID(c.Request.Context(), requestIdentity(c).UserID, releaseID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, releaseResponseFrom(result))
	}
}

func updateRelease(service Releases, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, ok := releaseIDParam(c)
		if !ok {
			return
		}
		var request updateReleaseRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		releaseDate, err := time.Parse(releaseDateLayout, request.ReleaseDate)
		if err != nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_release", "release_date must use YYYY-MM-DD")
			return
		}
		tracks := make([]release.TrackUpdateInput, 0, len(request.Tracks))
		for _, track := range request.Tracks {
			trackID, parseErr := uuid.Parse(track.ID)
			if parseErr != nil || trackID == uuid.Nil {
				writeError(c, http.StatusUnprocessableEntity, "invalid_release", "track id must be a UUID")
				return
			}
			tracks = append(tracks, release.TrackUpdateInput{
				ID: trackID, Title: track.Title, TrackNumber: track.TrackNumber,
				DiscNumber: track.DiscNumber, Explicit: track.Explicit,
			})
		}
		result, err := service.Update(c.Request.Context(), requestIdentity(c).UserID,
			releaseID, release.UpdateInput{
				Title: request.Title, Type: request.ReleaseType, ReleaseDate: releaseDate,
				CoverObjectKey: request.CoverObjectKey, Tracks: tracks,
			})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, releaseResponseFrom(result))
	}
}

func withdrawRelease(service Releases, logger *slog.Logger) gin.HandlerFunc {
	return releaseTransition(service, logger, false)
}

func archiveRelease(service Releases, logger *slog.Logger) gin.HandlerFunc {
	return releaseTransition(service, logger, true)
}

func releaseTransition(service Releases, logger *slog.Logger, archive bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseID, ok := releaseIDParam(c)
		if !ok {
			return
		}
		var result release.Release
		var err error
		if archive {
			result, err = service.Archive(c.Request.Context(), requestIdentity(c).UserID, releaseID)
		} else {
			result, err = service.Withdraw(c.Request.Context(), requestIdentity(c).UserID, releaseID)
		}
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, releaseResponseFrom(release.Draft{Release: result, Tracks: []release.Track{}}))
	}
}

func releaseIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("release_id"))
	if err != nil || id == uuid.Nil {
		writeError(c, http.StatusUnprocessableEntity, "invalid_release", "release_id must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}

func releaseResponseFrom(draft release.Draft) releaseResponse {
	result := releaseResponse{
		ID: draft.Release.ID.String(), CreatorID: draft.Release.CreatorID.String(),
		Title: draft.Release.Title, ReleaseType: draft.Release.Type,
		ReleaseDate: draft.Release.ReleaseDate.Format(releaseDateLayout),
		Status:      draft.Release.Status, CoverObjectKey: draft.Release.CoverObjectKey,
		SubmittedAt: draft.Release.SubmittedAt, ReviewedAt: draft.Release.ReviewedAt,
		PublishedAt: draft.Release.PublishedAt, ArchivedAt: draft.Release.ArchivedAt,
		CreatedAt: draft.Release.CreatedAt, UpdatedAt: draft.Release.UpdatedAt,
		Tracks: make([]releaseTrackResponse, 0, len(draft.Tracks)),
	}
	if draft.Release.CatalogAlbumID != nil {
		value := draft.Release.CatalogAlbumID.String()
		result.CatalogAlbumID = &value
	}
	for _, track := range draft.Tracks {
		response := releaseTrackResponse{
			ID: track.ID.String(), Title: track.Title, TrackNumber: track.TrackNumber,
			DiscNumber: track.DiscNumber, Explicit: track.Explicit,
		}
		if track.CatalogTrackID != nil {
			value := track.CatalogTrackID.String()
			response.CatalogTrackID = &value
		}
		result.Tracks = append(result.Tracks, response)
	}
	return result
}
