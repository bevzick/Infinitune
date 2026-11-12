package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type adminUserResponse struct {
	ID            uuid.UUID          `json:"id"`
	Email         string             `json:"email"`
	Status        auth.AccountStatus `json:"status"`
	EmailVerified bool               `json:"email_verified"`
	Roles         []string           `json:"roles"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	LastLoginAt   *time.Time         `json:"last_login_at"`
}

type adminUserPageResponse struct {
	Items []adminUserResponse `json:"items"`
	Page  pageResponse        `json:"page"`
}

type pageResponse struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

type updateUserStatusRequest struct {
	Status auth.AccountStatus `json:"status"`
}

func listUsers(service UserAdministration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		request, err := userPageRequest(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_user_query", err.Error())
			return
		}
		page, err := service.ListUsers(c.Request.Context(), actor, request)
		if err != nil {
			handleUserAdministrationError(c, logger, err)
			return
		}
		items := make([]adminUserResponse, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, presentAdminUser(item.User, item.Roles))
		}
		c.JSON(http.StatusOK, adminUserPageResponse{
			Items: items,
			Page:  pageResponse{Limit: page.Limit, Offset: page.Offset, Total: page.Total},
		})
	}
}

func updateUserStatus(service UserAdministration, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, userID, ok := roleRequestContext(c)
		if !ok {
			return
		}
		var request updateUserStatusRequest
		if !decodeJSON(c, &request) {
			return
		}
		user, err := service.UpdateUserStatus(c.Request.Context(), actor, userID, request.Status)
		if err != nil {
			handleUserAdministrationError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentAdminUser(user, nil))
	}
}

func userPageRequest(c *gin.Context) (auth.UserPageRequest, error) {
	for key, values := range c.Request.URL.Query() {
		switch key {
		case "status", "limit", "offset":
		default:
			return auth.UserPageRequest{}, fmt.Errorf("unsupported query parameter %q", key)
		}
		if len(values) != 1 {
			return auth.UserPageRequest{}, fmt.Errorf("query parameter %q must be provided once", key)
		}
	}
	request := auth.UserPageRequest{}
	if value, exists := c.GetQuery("status"); exists {
		if value == "" {
			return auth.UserPageRequest{}, fmt.Errorf("status must not be empty")
		}
		status := auth.AccountStatus(value)
		request.Status = &status
	}
	var err error
	if request.Limit, err = integerQuery(c, "limit"); err != nil {
		return auth.UserPageRequest{}, fmt.Errorf("limit must be an integer")
	}
	if request.Offset, err = integerQuery(c, "offset"); err != nil {
		return auth.UserPageRequest{}, fmt.Errorf("offset must be an integer")
	}
	return request, nil
}

func integerQuery(c *gin.Context, key string) (int, error) {
	value, exists := c.GetQuery(key)
	if !exists {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func presentAdminUser(user auth.User, roles []string) adminUserResponse {
	if roles == nil {
		roles = []string{}
	}
	return adminUserResponse{
		ID: user.ID, Email: user.Email, Status: user.Status,
		EmailVerified: user.EmailVerified, Roles: roles,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, LastLoginAt: user.LastLoginAt,
	}
}

func handleUserAdministrationError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		writeError(c, http.StatusForbidden, "insufficient_permissions", "Insufficient permissions")
	case errors.Is(err, auth.ErrInvalidUserQuery):
		writeError(c, http.StatusBadRequest, "invalid_user_query", "User query is invalid")
	case errors.Is(err, auth.ErrInvalidAccountStatus):
		writeError(c, http.StatusBadRequest, "invalid_account_status", "Status must be active or blocked")
	case errors.Is(err, auth.ErrUserNotFound):
		writeError(c, http.StatusNotFound, "user_not_found", "User was not found")
	case errors.Is(err, auth.ErrCannotChangeOwnStatus):
		writeError(c, http.StatusConflict, "admin_self_status_forbidden", "Admin cannot change their own status")
	case errors.Is(err, auth.ErrCannotDisableLastAdmin):
		writeError(c, http.StatusConflict, "last_active_admin_required", "The last active admin cannot be blocked")
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusGatewayTimeout, "request_timeout", "Request timed out")
	default:
		logger.ErrorContext(c.Request.Context(), "user administration failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
