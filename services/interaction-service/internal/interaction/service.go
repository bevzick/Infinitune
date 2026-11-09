package interaction

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const (
	defaultPageLimit = 50
	maximumPageLimit = 100
)

type Store interface {
	LikeTrack(context.Context, uuid.UUID, uuid.UUID) (Like, error)
	UnlikeTrack(context.Context, uuid.UUID, uuid.UUID) error
	TrackLikes(context.Context, uuid.UUID, PageRequest) (LikePage, error)
	LikeAlbum(context.Context, uuid.UUID, uuid.UUID) (Like, error)
	UnlikeAlbum(context.Context, uuid.UUID, uuid.UUID) error
	AlbumLikes(context.Context, uuid.UUID, PageRequest) (LikePage, error)
}

type CatalogValidator interface {
	ValidateTrack(context.Context, uuid.UUID) error
	ValidateAlbum(context.Context, uuid.UUID) error
}

type Service struct {
	store     Store
	validator CatalogValidator
}

func NewService(store Store, validator CatalogValidator) *Service {
	return &Service{store: store, validator: validator}
}

func (service *Service) LikeTrack(
	ctx context.Context,
	userID uuid.UUID,
	trackID uuid.UUID,
) (Like, error) {
	if err := validateIDs(userID, trackID, "track"); err != nil {
		return Like{}, err
	}
	if err := service.validator.ValidateTrack(ctx, trackID); err != nil {
		return Like{}, fmt.Errorf("validate track: %w", err)
	}
	liked, err := service.store.LikeTrack(ctx, userID, trackID)
	if err != nil {
		return Like{}, fmt.Errorf("like track: %w", err)
	}
	return liked, nil
}

func (service *Service) UnlikeTrack(ctx context.Context, userID, trackID uuid.UUID) error {
	if err := validateIDs(userID, trackID, "track"); err != nil {
		return err
	}
	if err := service.store.UnlikeTrack(ctx, userID, trackID); err != nil {
		return fmt.Errorf("unlike track: %w", err)
	}
	return nil
}

func (service *Service) TrackLikes(
	ctx context.Context,
	userID uuid.UUID,
	page PageRequest,
) (LikePage, error) {
	if userID == uuid.Nil {
		return LikePage{}, invalidArgument("user id is required")
	}
	page, err := validatePage(page)
	if err != nil {
		return LikePage{}, err
	}
	result, err := service.store.TrackLikes(ctx, userID, page)
	if err != nil {
		return LikePage{}, fmt.Errorf("list track likes: %w", err)
	}
	return result, nil
}

func (service *Service) LikeAlbum(
	ctx context.Context,
	userID uuid.UUID,
	albumID uuid.UUID,
) (Like, error) {
	if err := validateIDs(userID, albumID, "album"); err != nil {
		return Like{}, err
	}
	if err := service.validator.ValidateAlbum(ctx, albumID); err != nil {
		return Like{}, fmt.Errorf("validate album: %w", err)
	}
	liked, err := service.store.LikeAlbum(ctx, userID, albumID)
	if err != nil {
		return Like{}, fmt.Errorf("like album: %w", err)
	}
	return liked, nil
}

func (service *Service) UnlikeAlbum(ctx context.Context, userID, albumID uuid.UUID) error {
	if err := validateIDs(userID, albumID, "album"); err != nil {
		return err
	}
	if err := service.store.UnlikeAlbum(ctx, userID, albumID); err != nil {
		return fmt.Errorf("unlike album: %w", err)
	}
	return nil
}

func (service *Service) AlbumLikes(
	ctx context.Context,
	userID uuid.UUID,
	page PageRequest,
) (LikePage, error) {
	if userID == uuid.Nil {
		return LikePage{}, invalidArgument("user id is required")
	}
	page, err := validatePage(page)
	if err != nil {
		return LikePage{}, err
	}
	result, err := service.store.AlbumLikes(ctx, userID, page)
	if err != nil {
		return LikePage{}, fmt.Errorf("list album likes: %w", err)
	}
	return result, nil
}

func validateIDs(userID, targetID uuid.UUID, target string) error {
	if userID == uuid.Nil {
		return invalidArgument("user id is required")
	}
	if targetID == uuid.Nil {
		return invalidArgument(target + " id is required")
	}
	return nil
}

func validatePage(page PageRequest) (PageRequest, error) {
	if page.Limit == 0 {
		page.Limit = defaultPageLimit
	}
	if page.Limit < 1 || page.Limit > maximumPageLimit {
		return PageRequest{}, invalidArgument("page limit must be from 1 to 100")
	}
	if page.Offset < 0 {
		return PageRequest{}, invalidArgument("page offset must not be negative")
	}
	return page, nil
}
