package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultPageLimit = 20
	maximumPageLimit = 100
)

type recommendationItemResponse struct {
	TrackID uuid.UUID `json:"track_id"`
	Score   float64   `json:"score"`
	Rank    int       `json:"rank"`
	Reason  string    `json:"reason"`
}
type recommendationPageResponse struct {
	RunID            uuid.UUID                    `json:"run_id"`
	AlgorithmVersion string                       `json:"algorithm_version"`
	GeneratedAt      time.Time                    `json:"generated_at"`
	ExpiresAt        time.Time                    `json:"expires_at"`
	Items            []recommendationItemResponse `json:"items"`
	Total            int                          `json:"total"`
	Limit            int                          `json:"limit"`
	Offset           int                          `json:"offset"`
}

func listRecommendations(service Recommendation, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := requestIdentity(c)
		if !ok {
			writeUnauthorized(c)
			return
		}
		limit, offset, refresh, err := parseRecommendationQuery(c)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_recommendation_query", err.Error())
			return
		}
		run, err := service.Get(c.Request.Context(), actor.UserID, refresh)
		if err != nil {
			writeDomainError(c, logger, err)
			return
		}
		start := min(offset, len(run.Items))
		end := min(start+limit, len(run.Items))
		items := make([]recommendationItemResponse, 0, end-start)
		for _, item := range run.Items[start:end] {
			items = append(items, recommendationItemResponse{TrackID: item.TrackID, Score: item.Score, Rank: item.Rank, Reason: item.Reason})
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, recommendationPageResponse{
			RunID: run.ID, AlgorithmVersion: run.AlgorithmVersion, GeneratedAt: run.GeneratedAt, ExpiresAt: run.ExpiresAt,
			Items: items, Total: len(run.Items), Limit: limit, Offset: offset,
		})
	}
}

func parseRecommendationQuery(c *gin.Context) (int, int, bool, error) {
	allowed := map[string]struct{}{"limit": {}, "offset": {}, "refresh": {}}
	for key, values := range c.Request.URL.Query() {
		if _, ok := allowed[key]; !ok {
			return 0, 0, false, fmt.Errorf("unsupported query parameter %q", key)
		}
		if len(values) != 1 {
			return 0, 0, false, fmt.Errorf("query parameter %q must be provided once", key)
		}
	}
	limit, err := queryInt(c, "limit", defaultPageLimit)
	if err != nil || limit < 1 || limit > maximumPageLimit {
		return 0, 0, false, fmt.Errorf("limit must be an integer from 1 to %d", maximumPageLimit)
	}
	offset, err := queryInt(c, "offset", 0)
	if err != nil || offset < 0 {
		return 0, 0, false, fmt.Errorf("offset must be a non-negative integer")
	}
	refresh := false
	if value, exists := c.GetQuery("refresh"); exists {
		refresh, err = strconv.ParseBool(value)
		if err != nil {
			return 0, 0, false, fmt.Errorf("refresh must be true or false")
		}
	}
	return limit, offset, refresh, nil
}

func queryInt(c *gin.Context, key string, fallback int) (int, error) {
	value, exists := c.GetQuery(key)
	if !exists {
		return fallback, nil
	}
	return strconv.Atoi(value)
}
