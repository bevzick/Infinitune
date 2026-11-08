package playlist

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxNameLength = 150

type Store interface {
	Create(context.Context, Playlist) (Playlist, error)
	ByID(context.Context, uuid.UUID) (Playlist, error)
	Update(context.Context, Playlist) (Playlist, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) Create(
	ctx context.Context,
	actorID uuid.UUID,
	input CreateInput,
) (Playlist, error) {
	value, err := newPlaylist(actorID, input, TypeManual, nil)
	if err != nil {
		return Playlist{}, err
	}
	created, err := service.store.Create(ctx, value)
	if err != nil {
		return Playlist{}, fmt.Errorf("create playlist: %w", err)
	}
	return created, nil
}

func newPlaylist(
	actorID uuid.UUID,
	input CreateInput,
	playlistType Type,
	ruleSetID *uuid.UUID,
) (Playlist, error) {
	if actorID == uuid.Nil {
		return Playlist{}, invalidArgument("actor id is required")
	}
	name, err := validName(input.Name)
	if err != nil {
		return Playlist{}, err
	}
	visibility := input.Visibility
	if visibility == "" {
		visibility = VisibilityPrivate
	}
	if err = validateVisibility(visibility); err != nil {
		return Playlist{}, err
	}

	return Playlist{
		ID:          uuid.New(),
		OwnerID:     actorID,
		Name:        name,
		Description: input.Description,
		Type:        playlistType,
		Visibility:  visibility,
		RuleSetID:   ruleSetID,
	}, nil
}

func (service *Service) ByID(
	ctx context.Context,
	viewerID uuid.UUID,
	playlistID uuid.UUID,
) (Playlist, error) {
	if playlistID == uuid.Nil {
		return Playlist{}, invalidArgument("playlist id is required")
	}
	result, err := service.store.ByID(ctx, playlistID)
	if err != nil {
		return Playlist{}, fmt.Errorf("read playlist: %w", err)
	}
	if result.Visibility == VisibilityPrivate && result.OwnerID != viewerID {
		// Hide private playlist existence from callers who do not own it.
		return Playlist{}, ErrNotFound
	}
	return result, nil
}

func (service *Service) Update(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
	input UpdateInput,
) (Playlist, error) {
	current, err := service.owned(ctx, actorID, playlistID)
	if err != nil {
		return Playlist{}, err
	}
	changed := false
	if input.Name != nil {
		name, validationErr := validName(*input.Name)
		if validationErr != nil {
			return Playlist{}, validationErr
		}
		if name != current.Name {
			current.Name = name
			changed = true
		}
	}
	if input.Description != nil && *input.Description != current.Description {
		current.Description = *input.Description
		changed = true
	}
	if input.Visibility != nil {
		if validationErr := validateVisibility(*input.Visibility); validationErr != nil {
			return Playlist{}, validationErr
		}
		if *input.Visibility != current.Visibility {
			current.Visibility = *input.Visibility
			changed = true
		}
	}
	if !changed {
		return current, nil
	}

	updated, err := service.store.Update(ctx, current)
	if err != nil {
		return Playlist{}, fmt.Errorf("update playlist: %w", err)
	}
	return updated, nil
}

func (service *Service) Delete(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
) error {
	owned, err := service.owned(ctx, actorID, playlistID)
	if err != nil {
		return err
	}
	if err = service.store.Delete(ctx, owned.ID, owned.OwnerID); err != nil {
		return fmt.Errorf("delete playlist: %w", err)
	}
	return nil
}

func (service *Service) owned(
	ctx context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
) (Playlist, error) {
	if actorID == uuid.Nil {
		return Playlist{}, ErrForbidden
	}
	if playlistID == uuid.Nil {
		return Playlist{}, invalidArgument("playlist id is required")
	}
	result, err := service.store.ByID(ctx, playlistID)
	if err != nil {
		return Playlist{}, fmt.Errorf("read playlist ownership: %w", err)
	}
	if result.OwnerID != actorID {
		return Playlist{}, ErrForbidden
	}
	return result, nil
}

func validName(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", invalidArgument("playlist name is required")
	}
	if !utf8.ValidString(trimmed) || utf8.RuneCountInString(trimmed) > maxNameLength {
		return "", invalidArgument("playlist name must contain at most 150 characters")
	}
	return trimmed, nil
}

func validateVisibility(value Visibility) error {
	switch value {
	case VisibilityPrivate, VisibilityUnlisted, VisibilityPublic:
		return nil
	default:
		return invalidArgument("unsupported playlist visibility")
	}
}

func invalidArgument(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}
