package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maximumPageSize = 100
	dateLayout      = "2006-01-02"
)

type paginationResponse struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

type pageResponse[T any] struct {
	Items      []T                `json:"items"`
	Pagination paginationResponse `json:"pagination"`
}

type artistResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type artistSummaryResponse struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	ImageURL string    `json:"image_url"`
}

type albumArtistResponse struct {
	Artist   artistSummaryResponse `json:"artist"`
	Position int16                 `json:"position"`
}

type albumResponse struct {
	ID          uuid.UUID             `json:"id"`
	Title       string                `json:"title"`
	AlbumType   metadata.AlbumType    `json:"album_type"`
	ReleaseDate string                `json:"release_date"`
	CoverURL    string                `json:"cover_url"`
	Artists     []albumArtistResponse `json:"artists"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

type trackArtistResponse struct {
	Artist   artistSummaryResponse `json:"artist"`
	Role     metadata.ArtistRole   `json:"role"`
	Position int16                 `json:"position"`
}

type genreResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type moodResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type trackMoodResponse struct {
	Mood   moodResponse `json:"mood"`
	Weight float64      `json:"weight"`
}

type trackResponse struct {
	ID          uuid.UUID             `json:"id"`
	AlbumID     *uuid.UUID            `json:"album_id,omitempty"`
	Title       string                `json:"title"`
	DurationMS  int                   `json:"duration_ms"`
	TrackNumber int16                 `json:"track_number"`
	DiscNumber  int16                 `json:"disc_number"`
	Explicit    bool                  `json:"explicit"`
	ReleaseDate string                `json:"release_date"`
	Artists     []trackArtistResponse `json:"artists"`
	Genres      []genreResponse       `json:"genres"`
	Moods       []trackMoodResponse   `json:"moods"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

func listHandler[Model any, Response any](
	logger *slog.Logger,
	fetch func(context.Context, metadata.PageRequest) (metadata.Page[Model], error),
	convert func(Model) Response,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, ok := pageRequest(c)
		if !ok {
			return
		}
		result, err := fetch(c.Request.Context(), page)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]Response, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, convert(item))
		}
		c.JSON(http.StatusOK, pageResponse[Response]{
			Items: items,
			Pagination: paginationResponse{
				Limit: result.Limit, Offset: result.Offset, Total: result.Total,
			},
		})
	}
}

func detailHandler[Model any, Response any](
	logger *slog.Logger,
	fetch func(context.Context, uuid.UUID) (Model, error),
	convert func(Model) Response,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil || id == uuid.Nil {
			writeError(c, http.StatusBadRequest, "invalid_id", "ID must be a UUID", "id")
			return
		}
		result, err := fetch(c.Request.Context(), id)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, convert(result))
	}
}

func pageRequest(c *gin.Context) (metadata.PageRequest, bool) {
	limit, ok := paginationValue(c, "limit", defaultPageSize, 1, maximumPageSize)
	if !ok {
		return metadata.PageRequest{}, false
	}
	offset, ok := paginationValue(c, "offset", 0, 0, int(^uint(0)>>1))
	if !ok {
		return metadata.PageRequest{}, false
	}
	return metadata.PageRequest{Limit: limit, Offset: offset}, true
}

func paginationValue(c *gin.Context, name string, fallback, minimum, maximum int) (int, bool) {
	raw, exists := c.GetQuery(name)
	if !exists || raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		message := name + " must be a valid pagination value"
		if name == "limit" {
			message = "limit must be between 1 and 100"
		} else if name == "offset" {
			message = "offset must not be negative"
		}
		writeError(c, http.StatusBadRequest, "invalid_pagination", message, name)
		return 0, false
	}
	return value, true
}

func artistResponseFrom(artist metadata.Artist) artistResponse {
	return artistResponse{
		ID: artist.ID, Name: artist.Name, Description: artist.Description,
		ImageURL: artist.ImageURL, CreatedAt: artist.CreatedAt, UpdatedAt: artist.UpdatedAt,
	}
}

func artistSummaryResponseFrom(artist metadata.Artist) artistSummaryResponse {
	return artistSummaryResponse{ID: artist.ID, Name: artist.Name, ImageURL: artist.ImageURL}
}

func albumResponseFrom(album metadata.Album) albumResponse {
	artists := make([]albumArtistResponse, 0, len(album.Artists))
	for _, credit := range album.Artists {
		artists = append(artists, albumArtistResponse{
			Artist: artistSummaryResponseFrom(credit.Artist), Position: credit.Position,
		})
	}
	return albumResponse{
		ID: album.ID, Title: album.Title, AlbumType: album.Type,
		ReleaseDate: album.ReleaseDate.Format(dateLayout), CoverURL: album.CoverURL,
		Artists: artists, CreatedAt: album.CreatedAt, UpdatedAt: album.UpdatedAt,
	}
}

func trackResponseFrom(track metadata.Track) trackResponse {
	artists := make([]trackArtistResponse, 0, len(track.Artists))
	for _, credit := range track.Artists {
		artists = append(artists, trackArtistResponse{
			Artist: artistSummaryResponseFrom(credit.Artist), Role: credit.Role, Position: credit.Position,
		})
	}
	genres := make([]genreResponse, 0, len(track.Genres))
	for _, genre := range track.Genres {
		genres = append(genres, genreResponseFrom(genre))
	}
	moods := make([]trackMoodResponse, 0, len(track.Moods))
	for _, mood := range track.Moods {
		moods = append(moods, trackMoodResponse{
			Mood: moodResponseFrom(mood.Mood), Weight: mood.Weight,
		})
	}
	return trackResponse{
		ID: track.ID, AlbumID: track.AlbumID, Title: track.Title,
		DurationMS: track.DurationMS, TrackNumber: track.TrackNumber,
		DiscNumber: track.DiscNumber, Explicit: track.Explicit,
		ReleaseDate: track.ReleaseDate.Format(dateLayout), Artists: artists,
		Genres: genres, Moods: moods, CreatedAt: track.CreatedAt, UpdatedAt: track.UpdatedAt,
	}
}

func genreResponseFrom(genre metadata.Genre) genreResponse {
	return genreResponse{ID: genre.ID, Name: genre.Name, Slug: genre.Slug}
}

func moodResponseFrom(mood metadata.Mood) moodResponse {
	return moodResponse{ID: mood.ID, Name: mood.Name, Slug: mood.Slug}
}
