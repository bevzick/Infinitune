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
	LikeComment(context.Context, uuid.UUID, uuid.UUID) (Like, error)
	UnlikeComment(context.Context, uuid.UUID, uuid.UUID) error
	CommentLikes(context.Context, uuid.UUID, PageRequest) (LikePage, error)
	FollowArtist(context.Context, uuid.UUID, uuid.UUID) (Follow, error)
	UnfollowArtist(context.Context, uuid.UUID, uuid.UUID) error
	ArtistFollows(context.Context, uuid.UUID, PageRequest) (FollowPage, error)
}

type CatalogValidator interface {
	ValidateTrack(context.Context, uuid.UUID) error
	ValidateAlbum(context.Context, uuid.UUID) error
	ValidateArtist(context.Context, uuid.UUID) error
}

type CommentValidator interface {
	ValidateComment(context.Context, uuid.UUID) error
	ValidateComments(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

type Service struct {
	store     Store
	validator CatalogValidator
	comments  CommentValidator
}

func NewService(store Store, validator CatalogValidator, comments CommentValidator) *Service {
	return &Service{store: store, validator: validator, comments: comments}
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

func (service *Service) LikeComment(
	ctx context.Context,
	userID uuid.UUID,
	commentID uuid.UUID,
) (Like, error) {
	if err := validateIDs(userID, commentID, "comment"); err != nil {
		return Like{}, err
	}
	if err := service.comments.ValidateComment(ctx, commentID); err != nil {
		return Like{}, fmt.Errorf("validate comment: %w", err)
	}
	liked, err := service.store.LikeComment(ctx, userID, commentID)
	if err != nil {
		return Like{}, fmt.Errorf("like comment: %w", err)
	}
	return liked, nil
}

func (service *Service) UnlikeComment(ctx context.Context, userID, commentID uuid.UUID) error {
	if err := validateIDs(userID, commentID, "comment"); err != nil {
		return err
	}
	if err := service.store.UnlikeComment(ctx, userID, commentID); err != nil {
		return fmt.Errorf("unlike comment: %w", err)
	}
	return nil
}

func (service *Service) CommentLikes(
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
	result, err := service.store.CommentLikes(ctx, userID, page)
	if err != nil {
		return LikePage{}, fmt.Errorf("list comment likes: %w", err)
	}
	ids := make([]uuid.UUID, len(result.Items))
	for index, item := range result.Items {
		ids[index] = item.TargetID
	}
	visible, err := service.comments.ValidateComments(ctx, ids)
	if err != nil {
		return LikePage{}, fmt.Errorf("validate liked comments: %w", err)
	}
	items := result.Items[:0]
	for _, item := range result.Items {
		if _, ok := visible[item.TargetID]; ok {
			items = append(items, item)
		}
	}
	result.Items = items
	return result, nil
}

func (service *Service) FollowArtist(
	ctx context.Context,
	userID uuid.UUID,
	artistID uuid.UUID,
) (Follow, error) {
	if err := validateIDs(userID, artistID, "artist"); err != nil {
		return Follow{}, err
	}
	if err := service.validator.ValidateArtist(ctx, artistID); err != nil {
		return Follow{}, fmt.Errorf("validate artist: %w", err)
	}
	followed, err := service.store.FollowArtist(ctx, userID, artistID)
	if err != nil {
		return Follow{}, fmt.Errorf("follow artist: %w", err)
	}
	return followed, nil
}

func (service *Service) UnfollowArtist(ctx context.Context, userID, artistID uuid.UUID) error {
	if err := validateIDs(userID, artistID, "artist"); err != nil {
		return err
	}
	if err := service.store.UnfollowArtist(ctx, userID, artistID); err != nil {
		return fmt.Errorf("unfollow artist: %w", err)
	}
	return nil
}

func (service *Service) ArtistFollows(
	ctx context.Context,
	userID uuid.UUID,
	page PageRequest,
) (FollowPage, error) {
	if userID == uuid.Nil {
		return FollowPage{}, invalidArgument("user id is required")
	}
	page, err := validatePage(page)
	if err != nil {
		return FollowPage{}, err
	}
	result, err := service.store.ArtistFollows(ctx, userID, page)
	if err != nil {
		return FollowPage{}, fmt.Errorf("list artist follows: %w", err)
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
