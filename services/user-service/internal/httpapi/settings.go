package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/user-service/internal/profile"
	"github.com/gin-gonic/gin"
)

type updateSettingsRequest struct {
	Theme                 *profile.InterfaceTheme `json:"theme"`
	Autoplay              *bool                   `json:"autoplay"`
	ExplicitContent       *bool                   `json:"explicit_content"`
	AudioQuality          *profile.AudioQuality   `json:"audio_quality"`
	NormalizeVolume       *bool                   `json:"normalize_volume"`
	ShowListeningActivity *bool                   `json:"show_listening_activity"`
}

type settingsResponse struct {
	Theme                 profile.InterfaceTheme `json:"theme"`
	Autoplay              bool                   `json:"autoplay"`
	ExplicitContent       bool                   `json:"explicit_content"`
	AudioQuality          profile.AudioQuality   `json:"audio_quality"`
	NormalizeVolume       bool                   `json:"normalize_volume"`
	ShowListeningActivity bool                   `json:"show_listening_activity"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

func ownSettings(service Profiles, logger *slog.Logger) gin.HandlerFunc {
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
		c.JSON(http.StatusOK, newSettingsResponse(result.Settings))
	}
}

func updateSettings(service Profiles, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		verifiedIdentity, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		var request updateSettingsRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		updated, err := service.UpdateSettings(
			c.Request.Context(),
			verifiedIdentity.UserID,
			verifiedIdentity.UserID,
			profile.SettingsUpdateInput{
				Theme: request.Theme, Autoplay: request.Autoplay,
				ExplicitContent: request.ExplicitContent, AudioQuality: request.AudioQuality,
				NormalizeVolume:       request.NormalizeVolume,
				ShowListeningActivity: request.ShowListeningActivity,
			},
		)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, newSettingsResponse(updated))
	}
}

func newSettingsResponse(settings profile.Settings) settingsResponse {
	return settingsResponse{
		Theme: settings.Theme, Autoplay: settings.Autoplay,
		ExplicitContent: settings.ExplicitContent, AudioQuality: settings.AudioQuality,
		NormalizeVolume:       settings.NormalizeVolume,
		ShowListeningActivity: settings.ShowListeningActivity,
		CreatedAt:             settings.CreatedAt, UpdatedAt: settings.UpdatedAt,
	}
}
