package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RoleAssignments interface {
	ListUserRoles(context.Context, auth.Identity, uuid.UUID) ([]string, error)
	AssignRole(context.Context, auth.Identity, uuid.UUID, string) ([]string, error)
	RemoveRole(context.Context, auth.Identity, uuid.UUID, string) ([]string, error)
}

type userRolesResponse struct {
	UserID uuid.UUID `json:"user_id"`
	Roles  []string  `json:"roles"`
}

func listUserRoles(assignments RoleAssignments, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, userID, ok := roleRequestContext(c)
		if !ok {
			return
		}

		roles, err := assignments.ListUserRoles(c.Request.Context(), actor, userID)
		if err != nil {
			handleRoleError(c, logger, err)
			return
		}

		c.JSON(http.StatusOK, userRolesResponse{UserID: userID, Roles: roles})
	}
}

func assignUserRole(assignments RoleAssignments, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, userID, ok := roleRequestContext(c)
		if !ok {
			return
		}

		roles, err := assignments.AssignRole(
			c.Request.Context(),
			actor,
			userID,
			c.Param("role"),
		)
		if err != nil {
			handleRoleError(c, logger, err)
			return
		}

		c.JSON(http.StatusOK, userRolesResponse{UserID: userID, Roles: roles})
	}
}

func removeUserRole(assignments RoleAssignments, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, userID, ok := roleRequestContext(c)
		if !ok {
			return
		}

		roles, err := assignments.RemoveRole(
			c.Request.Context(),
			actor,
			userID,
			c.Param("role"),
		)
		if err != nil {
			handleRoleError(c, logger, err)
			return
		}

		c.JSON(http.StatusOK, userRolesResponse{UserID: userID, Roles: roles})
	}
}

func roleRequestContext(c *gin.Context) (auth.Identity, uuid.UUID, bool) {
	actor, ok := requestIdentity(c)
	if !ok {
		writeUnauthorized(c)
		return auth.Identity{}, uuid.Nil, false
	}

	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil || userID == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_user_id", "User ID must be a valid UUID")
		return auth.Identity{}, uuid.Nil, false
	}

	return actor, userID, true
}

func handleRoleError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		writeError(c, http.StatusForbidden, "insufficient_permissions", "Insufficient permissions")
	case errors.Is(err, auth.ErrInvalidRole):
		writeError(c, http.StatusBadRequest, "invalid_role", "Role is invalid")
	case errors.Is(err, auth.ErrProtectedRole):
		writeError(c, http.StatusBadRequest, "role_not_assignable", "Role is managed by the account lifecycle")
	case errors.Is(err, auth.ErrCannotRemoveOwnAdminRole):
		writeError(c, http.StatusConflict, "admin_self_removal_forbidden", "Admin cannot remove their own admin role")
	case errors.Is(err, auth.ErrCannotRemoveLastAdmin):
		writeError(c, http.StatusConflict, "last_admin_removal_forbidden", "The last admin role cannot be removed")
	case errors.Is(err, auth.ErrUserNotFound):
		writeError(c, http.StatusNotFound, "user_not_found", "User was not found")
	case errors.Is(err, context.Canceled):
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusGatewayTimeout, "request_timeout", "Request timed out")
	default:
		logger.ErrorContext(c.Request.Context(), "role operation failed", "error", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
