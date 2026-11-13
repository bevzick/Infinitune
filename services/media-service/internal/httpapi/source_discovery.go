package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/gin-gonic/gin"
)

type sourceProviderResponse struct {
	ID source.ProviderName `json:"id"`
}

type sourceTrackResponse struct {
	Provider   source.ProviderName   `json:"provider"`
	ExternalID string                `json:"external_id"`
	Title      string                `json:"title"`
	Artist     string                `json:"artist"`
	PageURL    string                `json:"page_url"`
	License    sourceLicenseResponse `json:"license"`
	Metadata   map[string]string     `json:"metadata"`
}

type sourceLicenseResponse struct {
	Name                 string `json:"name"`
	URL                  string `json:"url"`
	Attribution          string `json:"attribution"`
	AllowsStreaming      bool   `json:"allows_streaming"`
	AllowsRedistribution bool   `json:"allows_redistribution"`
}

type sourcePageResponse struct {
	Items  []sourceTrackResponse `json:"items"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
	Total  int                   `json:"total"`
}

func listSourceProviders(discovery SourceDiscovery) gin.HandlerFunc {
	return func(c *gin.Context) {
		if discovery == nil {
			writeError(c, http.StatusServiceUnavailable, "discovery_unavailable", "Source discovery is unavailable")
			return
		}
		providers := discovery.Providers()
		items := make([]sourceProviderResponse, 0, len(providers))
		for _, provider := range providers {
			items = append(items, sourceProviderResponse{ID: provider})
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func discoverSourceTracks(discovery SourceDiscovery, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if discovery == nil {
			writeError(c, http.StatusServiceUnavailable, "discovery_unavailable", "Source discovery is unavailable")
			return
		}
		limit, err := optionalInteger(c.Query("limit"), source.DefaultDiscoveryLimit)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_pagination", "Pagination parameters must be integers")
			return
		}
		offset, err := optionalInteger(c.Query("offset"), 0)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_pagination", "Pagination parameters must be integers")
			return
		}
		provider := source.ProviderName(c.Param("provider"))
		page, err := discovery.Discover(c.Request.Context(), provider, source.DiscoveryQuery{
			Query: c.Query("query"), Limit: limit, Offset: offset,
		})
		if err != nil {
			writeSourceDiscoveryError(c, logger, err)
			return
		}
		items := make([]sourceTrackResponse, 0, len(page.Tracks))
		for _, track := range page.Tracks {
			items = append(items, sourceTrack(provider, track))
		}
		c.JSON(http.StatusOK, sourcePageResponse{
			Items: items, Limit: page.Limit, Offset: page.Offset, Total: page.Total,
		})
	}
}

func resolveSourceTrack(discovery SourceDiscovery, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if discovery == nil {
			writeError(c, http.StatusServiceUnavailable, "discovery_unavailable", "Source discovery is unavailable")
			return
		}
		provider := source.ProviderName(c.Param("provider"))
		externalID := c.Query("external_id")
		track, err := discovery.Resolve(c.Request.Context(), provider, externalID)
		if err != nil {
			writeSourceDiscoveryError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, sourceTrack(provider, track))
	}
}

func sourceTrack(provider source.ProviderName, track source.Track) sourceTrackResponse {
	return sourceTrackResponse{
		Provider: provider, ExternalID: track.ExternalID, Title: track.Title,
		Artist: track.Artist, PageURL: track.PageURL, License: sourceLicenseResponse{
			Name: track.License.Name, URL: track.License.URL,
			Attribution:          track.License.Attribution,
			AllowsStreaming:      track.License.AllowsStreaming,
			AllowsRedistribution: track.License.AllowsRedistribution,
		},
		Metadata: track.Metadata,
	}
}

func optionalInteger(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func writeSourceDiscoveryError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, source.ErrUnsupportedProvider), errors.Is(err, source.ErrInvalid):
		writeError(c, http.StatusUnprocessableEntity, "invalid_source_query", err.Error())
	case errors.Is(err, source.ErrNotFound):
		writeError(c, http.StatusNotFound, "source_not_found", "External source was not found")
	case errors.Is(err, source.ErrProviderUnavailable):
		writeError(c, http.StatusServiceUnavailable, "discovery_unavailable", "Source provider is temporarily unavailable")
	default:
		loggerWithRequest(c, logger).Error("external source discovery failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "External source discovery failed")
	}
}
