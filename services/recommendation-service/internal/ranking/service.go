package ranking

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
	"github.com/google/uuid"
)

type SignalCollector interface {
	Collect(context.Context, uuid.UUID) (preferencesignal.Set, error)
}
type CandidateSource interface {
	Discover(context.Context, preferencesignal.Set) ([]Candidate, error)
	Candidates(context.Context, []uuid.UUID) ([]Candidate, error)
}

func (service *Service) RankCandidates(ctx context.Context, userID uuid.UUID, trackIDs []uuid.UUID) ([]recommendation.Item, error) {
	if userID == uuid.Nil || len(trackIDs) == 0 || len(trackIDs) > maximumLimit {
		return nil, fmt.Errorf("rank candidates: user id and 1 to 100 track ids are required")
	}
	signals, err := service.signals.Collect(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("collect ranking signals: %w", err)
	}
	candidates, err := service.candidates.Candidates(ctx, trackIDs)
	if err != nil {
		return nil, fmt.Errorf("load ranking candidates: %w", err)
	}
	return service.rankEligible(ctx, signals, candidates, len(trackIDs))
}

type Eligibility interface {
	Allowed(context.Context, uuid.UUID) (bool, error)
}

type Service struct {
	signals     SignalCollector
	candidates  CandidateSource
	eligibility Eligibility
}

func NewService(signals SignalCollector, candidates CandidateSource, eligibility Eligibility) *Service {
	return &Service{signals: signals, candidates: candidates, eligibility: eligibility}
}

func (service *Service) Rank(ctx context.Context, userID uuid.UUID, limit int) ([]recommendation.Item, error) {
	signals, err := service.signals.Collect(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("collect ranking signals: %w", err)
	}
	candidates, err := service.candidates.Discover(ctx, signals)
	if err != nil {
		return nil, fmt.Errorf("discover ranking candidates: %w", err)
	}
	desiredLimit := limit
	if desiredLimit == 0 {
		desiredLimit = defaultLimit
	}
	if desiredLimit < 1 || desiredLimit > maximumLimit {
		return nil, fmt.Errorf("rank candidates: limit must be from 1 to 100")
	}
	return service.rankEligible(ctx, signals, candidates, desiredLimit)
}

func (service *Service) rankEligible(ctx context.Context, signals preferencesignal.Set, candidates []Candidate, desiredLimit int) ([]recommendation.Item, error) {
	items := rankAll(signals, candidates)
	byTrackID := make(map[uuid.UUID]Candidate, len(candidates))
	for _, candidate := range candidates {
		byTrackID[candidate.TrackID] = candidate
	}
	eligible := make([]recommendation.Item, 0, desiredLimit)
	for _, item := range items {
		candidate, exists := byTrackID[item.TrackID]
		if !exists || candidate.ContentID == uuid.Nil {
			continue
		}
		allowed, allowErr := service.eligibility.Allowed(ctx, candidate.ContentID)
		if allowErr != nil {
			return nil, fmt.Errorf("check candidate playback rights: %w", allowErr)
		}
		if !allowed {
			continue
		}
		item.Rank = len(eligible) + 1
		eligible = append(eligible, item)
		if len(eligible) == desiredLimit {
			break
		}
	}
	return eligible, nil
}
