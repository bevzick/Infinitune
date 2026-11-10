package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createDynamicPlaylistRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Visibility  playlist.Visibility `json:"visibility"`
	Criteria    ruleSetRequest      `json:"criteria"`
}

type ruleSetRequest struct {
	Name       string                 `json:"name"`
	Logic      rules.LogicOperator    `json:"logic"`
	Conditions []ruleConditionRequest `json:"conditions"`
}

type ruleConditionRequest struct {
	Field    rules.Field      `json:"field"`
	Operator rules.Operator   `json:"operator"`
	Value    ruleValueRequest `json:"value"`
}

type ruleValueRequest struct {
	Kind         rules.ValueKind    `json:"kind"`
	Text         *string            `json:"text,omitempty"`
	UUID         *uuid.UUID         `json:"uuid,omitempty"`
	UUIDList     *[]uuid.UUID       `json:"uuid_list,omitempty"`
	Integer      *int               `json:"integer,omitempty"`
	IntegerRange *integerRangeValue `json:"integer_range,omitempty"`
	Boolean      *bool              `json:"boolean,omitempty"`
}

type integerRangeValue struct {
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}

type dynamicPlaylistResponse struct {
	Playlist playlistResponse `json:"playlist"`
	Criteria ruleSetResponse  `json:"criteria"`
}

type ruleSetResponse struct {
	ID         uuid.UUID               `json:"id"`
	OwnerID    uuid.UUID               `json:"owner_id"`
	Name       string                  `json:"name"`
	Logic      rules.LogicOperator     `json:"logic"`
	Conditions []ruleConditionResponse `json:"conditions"`
	CreatedAt  time.Time               `json:"created_at"`
	UpdatedAt  time.Time               `json:"updated_at"`
}

type ruleConditionResponse struct {
	ID       uuid.UUID         `json:"id"`
	Field    rules.Field       `json:"field"`
	Operator rules.Operator    `json:"operator"`
	Value    ruleValueResponse `json:"value"`
	Position int               `json:"position"`
}

type ruleValueResponse struct {
	Kind         rules.ValueKind    `json:"kind"`
	Text         *string            `json:"text,omitempty"`
	UUID         *uuid.UUID         `json:"uuid,omitempty"`
	UUIDList     *[]uuid.UUID       `json:"uuid_list,omitempty"`
	Integer      *int               `json:"integer,omitempty"`
	IntegerRange *integerRangeValue `json:"integer_range,omitempty"`
	Boolean      *bool              `json:"boolean,omitempty"`
}

type dynamicTrackResponse struct {
	TrackID  uuid.UUID `json:"track_id"`
	Position int       `json:"position"`
}

type dynamicResolutionResponse struct {
	Status playlist.DynamicResolutionStatus `json:"status"`
	Reason string                           `json:"reason,omitempty"`
}

type dynamicTrackPageResponse struct {
	Items        []dynamicTrackResponse    `json:"items"`
	Page         pageResponse              `json:"page"`
	Resolution   dynamicResolutionResponse `json:"resolution"`
	Personalized bool                      `json:"personalized"`
}

func createDynamicPlaylist(service DynamicPlaylists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		var request createDynamicPlaylistRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		criteria, err := request.Criteria.input()
		if err != nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_criteria", err.Error())
			return
		}
		created, err := service.Create(c.Request.Context(), actor.UserID, playlist.DynamicCreateInput{
			Playlist: playlist.CreateInput{
				Name: request.Name, Description: request.Description, Visibility: request.Visibility,
			},
			Criteria: criteria,
		})
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.Header("Location", "/v1/playlists/"+created.Playlist.ID.String())
		c.JSON(http.StatusCreated, dynamicPlaylistResponse{
			Playlist: playlistResponseFrom(created.Playlist),
			Criteria: ruleSetResponseFrom(created.RuleSet),
		})
	}
}

func getDynamicCriteria(service DynamicPlaylists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		playlistID, ok := pathUUID(c, "playlist_id")
		if !ok {
			return
		}
		criteria, err := service.Criteria(c.Request.Context(), actor.UserID, playlistID)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, ruleSetResponseFrom(criteria))
	}
}

func replaceDynamicCriteria(service DynamicPlaylists, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		playlistID, ok := pathUUID(c, "playlist_id")
		if !ok {
			return
		}
		var request ruleSetRequest
		if err := decodeJSON(c, &request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		input, err := request.input()
		if err != nil {
			writeError(c, http.StatusUnprocessableEntity, "invalid_criteria", err.Error())
			return
		}
		updated, err := service.ReplaceCriteria(c.Request.Context(), actor.UserID, playlistID, input)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, ruleSetResponseFrom(updated))
	}
}

func resolveDynamicTracks(service DynamicTracks, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		playlistID, ok := pathUUID(c, "playlist_id")
		if !ok {
			return
		}
		page, ok := trackPageRequest(c)
		if !ok {
			return
		}
		resolved, err := service.Resolve(c.Request.Context(), viewerID(c), playlistID, page)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		response := dynamicTrackPageResponse{
			Items: dynamicTracksResponse(resolved.Items),
			Page: pageResponse{
				Limit: resolved.Limit, Offset: resolved.Offset, Total: resolved.Total,
			},
			Resolution:   dynamicResolutionResponse{Status: resolved.Status},
			Personalized: resolved.Personalized,
		}
		if resolved.Status == playlist.DynamicResolutionEmpty {
			response.Resolution.Reason = "no_matches"
		}
		c.JSON(http.StatusOK, response)
	}
}

func (request ruleSetRequest) input() (rules.CreateInput, error) {
	conditions := make([]rules.ConditionInput, 0, len(request.Conditions))
	for index, condition := range request.Conditions {
		value, err := condition.Value.value()
		if err != nil {
			return rules.CreateInput{}, fmt.Errorf("conditions[%d].value: %w", index, err)
		}
		conditions = append(conditions, rules.ConditionInput{
			Field: condition.Field, Operator: condition.Operator, Value: value,
		})
	}
	return rules.CreateInput{Name: request.Name, Logic: request.Logic, Conditions: conditions}, nil
}

func (request ruleValueRequest) value() (rules.Value, error) {
	setValues := 0
	for _, isSet := range []bool{
		request.Text != nil, request.UUID != nil, request.UUIDList != nil,
		request.Integer != nil, request.IntegerRange != nil, request.Boolean != nil,
	} {
		if isSet {
			setValues++
		}
	}
	if setValues != 1 {
		return rules.Value{}, fmt.Errorf("kind %q must have exactly one typed value", request.Kind)
	}
	switch request.Kind {
	case rules.ValueText:
		if request.Text != nil {
			return rules.TextValue(*request.Text), nil
		}
	case rules.ValueUUID:
		if request.UUID != nil {
			return rules.UUIDValue(*request.UUID), nil
		}
	case rules.ValueUUIDList:
		if request.UUIDList != nil {
			return rules.UUIDListValue((*request.UUIDList)...), nil
		}
	case rules.ValueInteger:
		if request.Integer != nil {
			return rules.IntegerValue(*request.Integer), nil
		}
	case rules.ValueIntegerRange:
		if request.IntegerRange != nil {
			return rules.IntegerRangeValue(
				request.IntegerRange.Minimum, request.IntegerRange.Maximum,
			), nil
		}
	case rules.ValueBoolean:
		if request.Boolean != nil {
			return rules.BooleanValue(*request.Boolean), nil
		}
	default:
		return rules.Value{}, fmt.Errorf("kind %q is not supported", request.Kind)
	}
	return rules.Value{}, fmt.Errorf("kind %q does not match its typed value", request.Kind)
}

func ruleSetResponseFrom(value rules.RuleSet) ruleSetResponse {
	conditions := make([]ruleConditionResponse, 0, len(value.Conditions))
	for _, condition := range value.Conditions {
		conditions = append(conditions, ruleConditionResponse{
			ID: condition.ID, Field: condition.Field, Operator: condition.Operator,
			Value: ruleValueResponseFrom(condition.Value), Position: condition.Position,
		})
	}
	return ruleSetResponse{
		ID: value.ID, OwnerID: value.OwnerID, Name: value.Name, Logic: value.Logic,
		Conditions: conditions, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func ruleValueResponseFrom(value rules.Value) ruleValueResponse {
	result := ruleValueResponse{Kind: value.Kind}
	switch value.Kind {
	case rules.ValueText:
		result.Text = &value.Text
	case rules.ValueUUID:
		result.UUID = &value.UUID
	case rules.ValueUUIDList:
		values := append([]uuid.UUID(nil), value.UUIDs...)
		result.UUIDList = &values
	case rules.ValueInteger:
		result.Integer = &value.Integer
	case rules.ValueIntegerRange:
		result.IntegerRange = &integerRangeValue{
			Minimum: value.IntegerRange.Minimum, Maximum: value.IntegerRange.Maximum,
		}
	case rules.ValueBoolean:
		result.Boolean = &value.Boolean
	}
	return result
}

func dynamicTracksResponse(values []playlist.DynamicTrack) []dynamicTrackResponse {
	result := make([]dynamicTrackResponse, 0, len(values))
	for _, value := range values {
		result = append(result, dynamicTrackResponse{
			TrackID: value.TrackID, Position: value.Position,
		})
	}
	return result
}
