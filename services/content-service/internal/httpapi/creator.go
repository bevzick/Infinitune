package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type accountResponse struct {
	ID                 string                     `json:"id"`
	Type               creator.AccountType        `json:"type"`
	DisplayName        string                     `json:"display_name"`
	VerificationStatus creator.VerificationStatus `json:"verification_status"`
	CreatedAt          time.Time                  `json:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at"`
}
type memberResponse struct {
	UserID    string             `json:"user_id"`
	Role      creator.MemberRole `json:"role"`
	CreatedAt time.Time          `json:"created_at"`
}
type createRequest struct {
	Type        creator.AccountType `json:"type"`
	DisplayName string              `json:"display_name"`
}
type updateRequest struct {
	Type        *creator.AccountType `json:"type"`
	DisplayName *string              `json:"display_name"`
}
type reviewRequest struct {
	Decision creator.VerificationStatus `json:"decision"`
}
type memberRequest struct {
	UserID string             `json:"user_id"`
	Role   creator.MemberRole `json:"role"`
}
type roleRequest struct {
	Role creator.MemberRole `json:"role"`
}

func createCreator(service CreatorAccounts, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request createRequest
		if decodeJSON(c, &request) != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "Request body is invalid")
			return
		}
		account, err := service.Create(c.Request.Context(), requestIdentity(c).UserID, creator.CreateInput{Type: request.Type, DisplayName: request.DisplayName})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusCreated, presentAccount(account))
	}
}
func listCreators(service CreatorAccounts, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		accounts, err := service.List(c.Request.Context(), requestIdentity(c).UserID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		response := make([]accountResponse, 0, len(accounts))
		for _, account := range accounts {
			response = append(response, presentAccount(account))
		}
		c.JSON(http.StatusOK, response)
	}
}
func getCreator(service CreatorAccounts, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		account, err := service.ByID(c.Request.Context(), requestIdentity(c).UserID, creatorID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentAccount(account))
	}
}
func updateCreator(service CreatorAccounts, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		var request updateRequest
		if decodeJSON(c, &request) != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "Request body is invalid")
			return
		}
		account, err := service.Update(c.Request.Context(), requestIdentity(c).UserID, creatorID, creator.UpdateInput{Type: request.Type, DisplayName: request.DisplayName})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentAccount(account))
	}
}
func submitVerification(service CreatorAccounts, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		account, err := service.SubmitVerification(c.Request.Context(), requestIdentity(c).UserID, creatorID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentAccount(account))
	}
}
func reviewVerification(service CreatorAccounts, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		var request reviewRequest
		if decodeJSON(c, &request) != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "Request body is invalid")
			return
		}
		identity := requestIdentity(c)
		account, err := service.ReviewVerification(c.Request.Context(), creator.Actor{UserID: identity.UserID, Roles: identity.Roles}, creatorID, request.Decision)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentAccount(account))
	}
}
func listMembers(service CreatorMembers, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		members, err := service.List(c.Request.Context(), requestIdentity(c).UserID, creatorID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		response := make([]memberResponse, 0, len(members))
		for _, member := range members {
			response = append(response, presentMember(member))
		}
		c.JSON(http.StatusOK, response)
	}
}
func addMember(service CreatorMembers, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		var request memberRequest
		if decodeJSON(c, &request) != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "Request body is invalid")
			return
		}
		userID, err := uuid.Parse(request.UserID)
		if err != nil || userID == uuid.Nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_creator", "user_id must be a UUID")
			return
		}
		member, err := service.Add(c.Request.Context(), requestIdentity(c).UserID, creatorID, userID, request.Role)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusCreated, presentMember(member))
	}
}
func changeMemberRole(service CreatorMembers, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		userID, ok := pathID(c, "user_id")
		if !ok {
			return
		}
		var request roleRequest
		if decodeJSON(c, &request) != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "Request body is invalid")
			return
		}
		member, err := service.ChangeRole(c.Request.Context(), requestIdentity(c).UserID, creatorID, userID, request.Role)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, presentMember(member))
	}
}
func removeMember(service CreatorMembers, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		creatorID, ok := pathID(c, "creator_id")
		if !ok {
			return
		}
		userID, ok := pathID(c, "user_id")
		if !ok {
			return
		}
		if err := service.Remove(c.Request.Context(), requestIdentity(c).UserID, creatorID, userID); err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
func pathID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil || id == uuid.Nil {
		writeError(c, http.StatusUnprocessableEntity, "invalid_creator", name+" must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}
func presentAccount(value creator.Account) accountResponse {
	return accountResponse{ID: value.ID.String(), Type: value.Type, DisplayName: value.DisplayName, VerificationStatus: value.VerificationStatus, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func presentMember(value creator.Member) memberResponse {
	return memberResponse{UserID: value.UserID.String(), Role: value.Role, CreatedAt: value.CreatedAt}
}
