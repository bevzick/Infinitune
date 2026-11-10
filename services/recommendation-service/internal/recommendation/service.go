package recommendation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const generatedItemLimit = 100

type Ranker interface {
	Rank(context.Context, uuid.UUID, int) ([]Item, error)
}

type RunStore interface {
	LatestValid(context.Context, uuid.UUID, string, time.Time) (Run, error)
	Save(context.Context, Run) (Run, error)
	SaveIfNoValid(context.Context, Run, time.Time) (Run, bool, error)
}

type Service struct {
	ranker           Ranker
	store            RunStore
	algorithmVersion string
	ttl              time.Duration
	now              func() time.Time
	newID            func() uuid.UUID
}

func NewService(ranker Ranker, store RunStore, algorithmVersion string, ttl time.Duration) (*Service, error) {
	algorithmVersion = strings.TrimSpace(algorithmVersion)
	if ranker == nil || store == nil || algorithmVersion == "" || ttl <= 0 {
		return nil, ErrInvalid
	}
	return &Service{
		ranker: ranker, store: store, algorithmVersion: algorithmVersion, ttl: ttl,
		now: time.Now, newID: uuid.New,
	}, nil
}

// Get returns the current versioned run. A normal request reuses a valid run;
// refresh deliberately bypasses the cache and produces a new one.
func (service *Service) Get(ctx context.Context, userID uuid.UUID, refresh bool) (Run, error) {
	if userID == uuid.Nil {
		return Run{}, ErrInvalid
	}
	lookupAt := service.now().UTC()
	if !refresh {
		current, err := service.store.LatestValid(ctx, userID, service.algorithmVersion, lookupAt)
		if err == nil {
			return current, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Run{}, fmt.Errorf("read cached recommendation run: %w", err)
		}
	}

	items, err := service.ranker.Rank(ctx, userID, generatedItemLimit)
	if err != nil {
		return Run{}, fmt.Errorf("rank recommendation run: %w", err)
	}
	generatedAt := service.now().UTC()
	candidate, err := NewRun(
		service.newID(), userID, service.algorithmVersion,
		generatedAt, generatedAt.Add(service.ttl), items,
	)
	if err != nil {
		return Run{}, fmt.Errorf("build recommendation run: %w", err)
	}
	if refresh {
		stored, saveErr := service.store.Save(ctx, candidate)
		if saveErr != nil {
			return Run{}, fmt.Errorf("save refreshed recommendation run: %w", saveErr)
		}
		return stored, nil
	}
	stored, _, err := service.store.SaveIfNoValid(ctx, candidate, generatedAt)
	if err != nil {
		return Run{}, fmt.Errorf("save idempotent recommendation run: %w", err)
	}
	return stored, nil
}
