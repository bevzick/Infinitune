package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type releasePageMetadata struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

type releasePageResponse struct {
	Items []releaseResponse   `json:"items"`
	Page  releasePageMetadata `json:"page"`
}

func listReleases(service Releases, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		request, err := releasePageRequest(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_release_query", err.Error())
			return
		}
		result, err := service.List(c.Request.Context(), requestIdentity(c).UserID, request)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentReleasePage(result))
	}
}

func listReleaseModerationQueue(service Releases, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		request, err := releasePageRequest(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_release_query", err.Error())
			return
		}
		identity := requestIdentity(c)
		result, err := service.ListModeration(c.Request.Context(), creator.Actor{
			UserID: identity.UserID, Roles: identity.Roles,
		}, request)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentReleasePage(result))
	}
}

func releasePageRequest(c *gin.Context) (release.PageRequest, error) {
	for key, values := range c.Request.URL.Query() {
		switch key {
		case "creator_id", "status", "limit", "offset":
		default:
			return release.PageRequest{}, fmt.Errorf("unsupported query parameter %q", key)
		}
		if len(values) != 1 {
			return release.PageRequest{}, fmt.Errorf("query parameter %q must be provided once", key)
		}
	}
	request := release.PageRequest{}
	if value, exists := c.GetQuery("creator_id"); exists {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return release.PageRequest{}, fmt.Errorf("creator_id must be a UUID")
		}
		request.CreatorID = &id
	}
	if value, exists := c.GetQuery("status"); exists {
		if value == "" {
			return release.PageRequest{}, fmt.Errorf("status must not be empty")
		}
		status := release.Status(value)
		request.Status = &status
	}
	var err error
	if request.Limit, err = queryInteger(c, "limit"); err != nil {
		return release.PageRequest{}, fmt.Errorf("limit must be an integer")
	}
	if request.Offset, err = queryInteger(c, "offset"); err != nil {
		return release.PageRequest{}, fmt.Errorf("offset must be an integer")
	}
	return request, nil
}

func queryInteger(c *gin.Context, key string) (int, error) {
	value, exists := c.GetQuery(key)
	if !exists {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func presentReleasePage(page release.Page) releasePageResponse {
	items := make([]releaseResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, releaseResponseFrom(release.Draft{
			Release: item, Tracks: []release.Track{},
		}))
	}
	return releasePageResponse{
		Items: items,
		Page:  releasePageMetadata{Limit: page.Limit, Offset: page.Offset, Total: page.Total},
	}
}
