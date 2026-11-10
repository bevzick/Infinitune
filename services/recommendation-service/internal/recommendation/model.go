package recommendation

import (
	"errors"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalid  = errors.New("recommendation run is invalid")
	ErrNotFound = errors.New("recommendation run not found")
	ErrConflict = errors.New("recommendation run already exists")
)

type Run struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	AlgorithmVersion string
	GeneratedAt      time.Time
	ExpiresAt        time.Time
	Items            []Item
}

type Item struct {
	RunID   uuid.UUID
	TrackID uuid.UUID
	Score   float64
	Rank    int
	Reason  string
}

func NewRun(
	id uuid.UUID,
	userID uuid.UUID,
	algorithmVersion string,
	generatedAt time.Time,
	expiresAt time.Time,
	items []Item,
) (Run, error) {
	if id == uuid.Nil || userID == uuid.Nil {
		return Run{}, ErrInvalid
	}
	algorithmVersion = strings.TrimSpace(algorithmVersion)
	if algorithmVersion == "" || !utf8.ValidString(algorithmVersion) ||
		utf8.RuneCountInString(algorithmVersion) > 50 {
		return Run{}, ErrInvalid
	}
	if generatedAt.IsZero() || expiresAt.IsZero() || !expiresAt.After(generatedAt) {
		return Run{}, ErrInvalid
	}
	canonical := make([]Item, len(items))
	trackIDs := make(map[uuid.UUID]struct{}, len(items))
	ranks := make(map[int]struct{}, len(items))
	for index, item := range items {
		reason := strings.TrimSpace(item.Reason)
		if item.TrackID == uuid.Nil || math.IsNaN(item.Score) || math.IsInf(item.Score, 0) ||
			item.Rank < 1 || item.Rank > len(items) || reason == "" ||
			!utf8.ValidString(reason) || utf8.RuneCountInString(reason) > 100 {
			return Run{}, ErrInvalid
		}
		if _, exists := trackIDs[item.TrackID]; exists {
			return Run{}, ErrInvalid
		}
		if _, exists := ranks[item.Rank]; exists {
			return Run{}, ErrInvalid
		}
		trackIDs[item.TrackID] = struct{}{}
		ranks[item.Rank] = struct{}{}
		canonical[index] = Item{
			RunID: id, TrackID: item.TrackID, Score: item.Score,
			Rank: item.Rank, Reason: reason,
		}
	}
	slices.SortFunc(canonical, func(left, right Item) int {
		return left.Rank - right.Rank
	})
	return Run{
		ID: id, UserID: userID, AlgorithmVersion: algorithmVersion,
		GeneratedAt: generatedAt.UTC(), ExpiresAt: expiresAt.UTC(), Items: canonical,
	}, nil
}
