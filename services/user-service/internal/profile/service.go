package profile

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	Create(context.Context, CreateInput) (Profile, error)
	ByUserID(context.Context, uuid.UUID) (Profile, error)
	Update(context.Context, Profile) (Profile, error)
	UpdateSettings(context.Context, uuid.UUID, Settings) (Settings, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (service *Service) Create(
	ctx context.Context,
	actorUserID uuid.UUID,
	input CreateInput,
) (Profile, error) {
	if !ownsProfile(actorUserID, input.UserID) {
		return Profile{}, ErrForbidden
	}

	username, err := validateUsername(input.Username)
	if err != nil {
		return Profile{}, err
	}
	displayName, err := validateDisplayName(input.DisplayName)
	if err != nil {
		return Profile{}, err
	}

	created, err := service.store.Create(ctx, CreateInput{
		UserID:      input.UserID,
		Username:    username,
		DisplayName: displayName,
	})
	if err != nil {
		return Profile{}, fmt.Errorf("create profile: %w", err)
	}

	return created, nil
}

func (service *Service) Own(
	ctx context.Context,
	actorUserID uuid.UUID,
	profileUserID uuid.UUID,
) (Profile, error) {
	if !ownsProfile(actorUserID, profileUserID) {
		return Profile{}, ErrForbidden
	}

	result, err := service.store.ByUserID(ctx, profileUserID)
	if err != nil {
		return Profile{}, fmt.Errorf("read own profile: %w", err)
	}

	return result, nil
}

func (service *Service) Public(ctx context.Context, userID uuid.UUID) (PublicProfile, error) {
	result, err := service.store.ByUserID(ctx, userID)
	if err != nil {
		return PublicProfile{}, fmt.Errorf("read public profile: %w", err)
	}

	return PublicProfile{
		UserID:      result.UserID,
		Username:    result.Username,
		DisplayName: result.DisplayName,
		Bio:         result.Bio,
		AvatarURL:   result.AvatarURL,
		CountryCode: result.CountryCode,
		Language:    result.Language,
		CreatedAt:   result.CreatedAt,
		UpdatedAt:   result.UpdatedAt,
	}, nil
}

func (service *Service) Update(
	ctx context.Context,
	actorUserID uuid.UUID,
	profileUserID uuid.UUID,
	input UpdateInput,
) (Profile, error) {
	if !ownsProfile(actorUserID, profileUserID) {
		return Profile{}, ErrForbidden
	}

	current, err := service.store.ByUserID(ctx, profileUserID)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile for update: %w", err)
	}

	updated, changed, err := applyUpdate(current, input, service.now().UTC())
	if err != nil {
		return Profile{}, err
	}
	if !changed {
		return current, nil
	}

	result, err := service.store.Update(ctx, updated)
	if err != nil {
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}

	return result, nil
}

func (service *Service) UpdateSettings(
	ctx context.Context,
	actorUserID uuid.UUID,
	profileUserID uuid.UUID,
	input SettingsUpdateInput,
) (Settings, error) {
	if !ownsProfile(actorUserID, profileUserID) {
		return Settings{}, ErrForbidden
	}

	current, err := service.store.ByUserID(ctx, profileUserID)
	if err != nil {
		return Settings{}, fmt.Errorf("read settings for update: %w", err)
	}

	updated, changed, err := applySettingsUpdate(current.Settings, input)
	if err != nil {
		return Settings{}, err
	}
	if !changed {
		return current.Settings, nil
	}

	result, err := service.store.UpdateSettings(ctx, profileUserID, updated)
	if err != nil {
		return Settings{}, fmt.Errorf("update settings: %w", err)
	}

	return result, nil
}

func ownsProfile(actorUserID, profileUserID uuid.UUID) bool {
	return actorUserID != uuid.Nil && actorUserID == profileUserID
}
