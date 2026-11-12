package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/gin-gonic/gin"
)

type accountPageResponse struct {
	Items []accountResponse   `json:"items"`
	Page  releasePageMetadata `json:"page"`
}

func listCreatorVerificationQueue(
	service CreatorAccounts,
	logger *slog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		request, err := creatorVerificationPageRequest(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_creator_query", err.Error())
			return
		}
		identity := requestIdentity(c)
		result, err := service.ListVerification(c.Request.Context(), creator.Actor{
			UserID: identity.UserID, Roles: identity.Roles,
		}, request)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		items := make([]accountResponse, 0, len(result.Items))
		for _, account := range result.Items {
			items = append(items, presentAccount(account))
		}
		c.JSON(http.StatusOK, accountPageResponse{
			Items: items,
			Page: releasePageMetadata{
				Limit: result.Limit, Offset: result.Offset, Total: result.Total,
			},
		})
	}
}

func creatorVerificationPageRequest(c *gin.Context) (creator.VerificationPageRequest, error) {
	for key, values := range c.Request.URL.Query() {
		switch key {
		case "status", "limit", "offset":
		default:
			return creator.VerificationPageRequest{}, fmt.Errorf("unsupported query parameter %q", key)
		}
		if len(values) != 1 {
			return creator.VerificationPageRequest{}, fmt.Errorf("query parameter %q must be provided once", key)
		}
	}
	request := creator.VerificationPageRequest{}
	if value, exists := c.GetQuery("status"); exists {
		if value == "" {
			return creator.VerificationPageRequest{}, fmt.Errorf("status must not be empty")
		}
		status := creator.VerificationStatus(value)
		request.Status = &status
	}
	var err error
	if request.Limit, err = queryInteger(c, "limit"); err != nil {
		return creator.VerificationPageRequest{}, fmt.Errorf("limit must be an integer")
	}
	if request.Offset, err = queryInteger(c, "offset"); err != nil {
		return creator.VerificationPageRequest{}, fmt.Errorf("offset must be an integer")
	}
	return request, nil
}
