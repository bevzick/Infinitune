package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const dateLayout = "2006-01-02"

type createProfileRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type updateProfileRequest struct {
	Username    *string      `json:"username"`
	DisplayName *string      `json:"display_name"`
	Bio         *string      `json:"bio"`
	AvatarURL   *string      `json:"avatar_url"`
	CountryCode *string      `json:"country_code"`
	Language    *string      `json:"language"`
	BirthDate   nullableDate `json:"birth_date"`
}

type nullableDate struct {
	Set   bool
	Value *time.Time
}

func (date *nullableDate) UnmarshalJSON(data []byte) error {
	date.Set = true
	if bytes.Equal(data, []byte("null")) {
		date.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("birth_date must be a date string or null")
	}
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return errors.New("birth_date must use YYYY-MM-DD format")
	}
	date.Value = &parsed
	return nil
}

type profileResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	AvatarURL   string    `json:"avatar_url"`
	CountryCode string    `json:"country_code"`
	Language    string    `json:"language"`
	BirthDate   *string   `json:"birth_date,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func createProfile(service Profiles, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		verifiedIdentity, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		var request createProfileRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		created, err := service.Create(c.Request.Context(), verifiedIdentity.UserID, profile.CreateInput{
			UserID:      verifiedIdentity.UserID,
			Username:    request.Username,
			DisplayName: request.DisplayName,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusCreated, ownProfileResponse(created))
	}
}

func ownProfile(service Profiles, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		verifiedIdentity, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		result, err := service.Own(c.Request.Context(), verifiedIdentity.UserID, verifiedIdentity.UserID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, ownProfileResponse(result))
	}
}

func publicProfile(service Profiles, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.Param("user_id"))
		if err != nil || userID == uuid.Nil {
			writeError(c, http.StatusBadRequest, "invalid_user_id", "User ID must be a UUID")
			return
		}
		result, err := service.Public(c.Request.Context(), userID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, profileResponse{
			UserID: result.UserID, Username: result.Username, DisplayName: result.DisplayName,
			Bio: result.Bio, AvatarURL: result.AvatarURL, CountryCode: result.CountryCode,
			Language: result.Language, CreatedAt: result.CreatedAt, UpdatedAt: result.UpdatedAt,
		})
	}
}

func updateProfile(service Profiles, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		verifiedIdentity, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		var request updateProfileRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		updated, err := service.Update(
			c.Request.Context(),
			verifiedIdentity.UserID,
			verifiedIdentity.UserID,
			profile.UpdateInput{
				Username: request.Username, DisplayName: request.DisplayName, Bio: request.Bio,
				AvatarURL: request.AvatarURL, CountryCode: request.CountryCode, Language: request.Language,
				BirthDate: profile.BirthDateUpdate{Set: request.BirthDate.Set, Value: request.BirthDate.Value},
			},
		)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, ownProfileResponse(updated))
	}
}

func ownProfileResponse(result profile.Profile) profileResponse {
	var birthDate *string
	if result.BirthDate != nil {
		formatted := result.BirthDate.Format(dateLayout)
		birthDate = &formatted
	}
	return profileResponse{
		UserID: result.UserID, Username: result.Username, DisplayName: result.DisplayName,
		Bio: result.Bio, AvatarURL: result.AvatarURL, CountryCode: result.CountryCode,
		Language: result.Language, BirthDate: birthDate,
		CreatedAt: result.CreatedAt, UpdatedAt: result.UpdatedAt,
	}
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
