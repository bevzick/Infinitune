package interaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceLikesValidatedTrackAndAlbumForVerifiedOwner(t *testing.T) {
	userID, trackID, albumID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	store := &storeStub{like: Like{CreatedAt: now}}
	validator := &validatorStub{}
	service := NewService(store, validator, &commentValidatorStub{})

	trackLike, err := service.LikeTrack(context.Background(), userID, trackID)
	if err != nil || trackLike.UserID != userID || trackLike.TargetID != trackID {
		t.Fatalf("LikeTrack() = %+v, %v", trackLike, err)
	}
	albumLike, err := service.LikeAlbum(context.Background(), userID, albumID)
	if err != nil || albumLike.UserID != userID || albumLike.TargetID != albumID {
		t.Fatalf("LikeAlbum() = %+v, %v", albumLike, err)
	}
	if validator.trackID != trackID || validator.albumID != albumID {
		t.Fatalf("validated track %s and album %s", validator.trackID, validator.albumID)
	}
	if store.userID != userID {
		t.Fatalf("stored owner = %s, want %s", store.userID, userID)
	}
}

func TestServiceDoesNotStoreUnavailableCatalogTarget(t *testing.T) {
	store := &storeStub{}
	service := NewService(store, &validatorStub{trackErr: ErrTrackNotFound}, &commentValidatorStub{})
	_, err := service.LikeTrack(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("LikeTrack() error = %v, want %v", err, ErrTrackNotFound)
	}
	if store.likeCalls != 0 {
		t.Fatalf("store like calls = %d, want 0", store.likeCalls)
	}
}

func TestServiceUnlikeIsIdempotentAndScopedToOwner(t *testing.T) {
	userID, trackID := uuid.New(), uuid.New()
	store := &storeStub{}
	service := NewService(store, &validatorStub{}, &commentValidatorStub{})
	for range 2 {
		if err := service.UnlikeTrack(context.Background(), userID, trackID); err != nil {
			t.Fatalf("UnlikeTrack() error = %v", err)
		}
	}
	if store.userID != userID || store.targetID != trackID || store.unlikeCalls != 2 {
		t.Fatalf("unlike owner %s target %s calls %d", store.userID, store.targetID, store.unlikeCalls)
	}
}

func TestServiceListUsesDefaultsAndRejectsInvalidPagination(t *testing.T) {
	userID := uuid.New()
	store := &storeStub{}
	service := NewService(store, &validatorStub{}, &commentValidatorStub{})
	result, err := service.TrackLikes(context.Background(), userID, PageRequest{})
	if err != nil || result.Limit != 50 || store.page != (PageRequest{Limit: 50}) {
		t.Fatalf("TrackLikes() = %+v, %v; page = %+v", result, err, store.page)
	}
	if _, err = service.AlbumLikes(context.Background(), userID, PageRequest{Limit: 101}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("AlbumLikes() error = %v, want %v", err, ErrInvalidArgument)
	}
}

func TestServiceFollowsValidatedArtistsForVerifiedOwner(t *testing.T) {
	userID, firstArtistID, secondArtistID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{}
	validator := &validatorStub{}
	service := NewService(store, validator, &commentValidatorStub{})
	for _, artistID := range []uuid.UUID{firstArtistID, secondArtistID} {
		followed, err := service.FollowArtist(context.Background(), userID, artistID)
		if err != nil || followed.UserID != userID || followed.ArtistID != artistID {
			t.Fatalf("FollowArtist() = %+v, %v", followed, err)
		}
	}
	if store.followCalls != 2 || store.userID != userID || store.targetID != secondArtistID ||
		validator.artistID != secondArtistID {
		t.Fatalf("follow calls = %d, owner = %s, artist = %s, validated = %s",
			store.followCalls, store.userID, store.targetID, validator.artistID)
	}
}

func TestServiceDoesNotFollowMissingArtist(t *testing.T) {
	store := &storeStub{}
	service := NewService(store, &validatorStub{artistErr: ErrArtistNotFound}, &commentValidatorStub{})
	_, err := service.FollowArtist(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrArtistNotFound) || store.followCalls != 0 {
		t.Fatalf("FollowArtist() error = %v, store calls = %d", err, store.followCalls)
	}
}

func TestServiceLikesOnlyVisibleCommentsAndAllowsIdempotentUnlike(t *testing.T) {
	userID, commentID := uuid.New(), uuid.New()
	store := &storeStub{}
	validator := &commentValidatorStub{}
	service := NewService(store, &validatorStub{}, validator)
	liked, err := service.LikeComment(context.Background(), userID, commentID)
	if err != nil || liked.UserID != userID || liked.TargetID != commentID || validator.commentID != commentID {
		t.Fatalf("LikeComment() = %+v, %v; validated = %s", liked, err, validator.commentID)
	}
	validator.err = ErrCommentNotFound
	if _, err = service.LikeComment(context.Background(), userID, uuid.New()); !errors.Is(err, ErrCommentNotFound) || store.likeCalls != 1 {
		t.Fatalf("LikeComment(hidden) error/calls = %v/%d", err, store.likeCalls)
	}
	for range 2 {
		if err = service.UnlikeComment(context.Background(), userID, commentID); err != nil {
			t.Fatalf("UnlikeComment() error = %v", err)
		}
	}
}

func TestServiceFiltersUnavailableCommentLikes(t *testing.T) {
	userID, visibleID, hiddenID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{likePage: LikePage{
		Items: []Like{{UserID: userID, TargetID: visibleID}, {UserID: userID, TargetID: hiddenID}},
		Limit: 10, Total: 2,
	}}
	validator := &commentValidatorStub{visible: map[uuid.UUID]struct{}{visibleID: {}}}
	result, err := NewService(store, &validatorStub{}, validator).CommentLikes(
		context.Background(), userID, PageRequest{Limit: 10},
	)
	if err != nil || len(result.Items) != 1 || result.Items[0].TargetID != visibleID {
		t.Fatalf("CommentLikes() = %+v, %v", result, err)
	}
}

type validatorStub struct {
	trackID   uuid.UUID
	albumID   uuid.UUID
	artistID  uuid.UUID
	trackErr  error
	albumErr  error
	artistErr error
}

type commentValidatorStub struct {
	commentID uuid.UUID
	visible   map[uuid.UUID]struct{}
	err       error
}

func (stub *commentValidatorStub) ValidateComment(_ context.Context, id uuid.UUID) error {
	stub.commentID = id
	return stub.err
}

func (stub *commentValidatorStub) ValidateComments(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	if stub.visible != nil {
		return stub.visible, stub.err
	}
	result := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, stub.err
}

func (stub *validatorStub) ValidateTrack(_ context.Context, id uuid.UUID) error {
	stub.trackID = id
	return stub.trackErr
}

func (stub *validatorStub) ValidateAlbum(_ context.Context, id uuid.UUID) error {
	stub.albumID = id
	return stub.albumErr
}

func (stub *validatorStub) ValidateArtist(_ context.Context, id uuid.UUID) error {
	stub.artistID = id
	return stub.artistErr
}

type storeStub struct {
	like          Like
	userID        uuid.UUID
	targetID      uuid.UUID
	page          PageRequest
	likePage      LikePage
	likeCalls     int
	unlikeCalls   int
	followCalls   int
	unfollowCalls int
}

func (stub *storeStub) liked(userID, targetID uuid.UUID) (Like, error) {
	stub.userID, stub.targetID = userID, targetID
	stub.likeCalls++
	result := stub.like
	result.UserID, result.TargetID = userID, targetID
	return result, nil
}

func (stub *storeStub) unliked(userID, targetID uuid.UUID) error {
	stub.userID, stub.targetID = userID, targetID
	stub.unlikeCalls++
	return nil
}

func (stub *storeStub) listed(userID uuid.UUID, page PageRequest) (LikePage, error) {
	stub.userID, stub.page = userID, page
	if stub.likePage.Items != nil {
		return stub.likePage, nil
	}
	return LikePage{Items: []Like{}, Limit: page.Limit, Offset: page.Offset}, nil
}

func (stub *storeStub) LikeTrack(_ context.Context, userID, id uuid.UUID) (Like, error) {
	return stub.liked(userID, id)
}
func (stub *storeStub) UnlikeTrack(_ context.Context, userID, id uuid.UUID) error {
	return stub.unliked(userID, id)
}
func (stub *storeStub) TrackLikes(_ context.Context, userID uuid.UUID, page PageRequest) (LikePage, error) {
	return stub.listed(userID, page)
}
func (stub *storeStub) LikeAlbum(_ context.Context, userID, id uuid.UUID) (Like, error) {
	return stub.liked(userID, id)
}
func (stub *storeStub) UnlikeAlbum(_ context.Context, userID, id uuid.UUID) error {
	return stub.unliked(userID, id)
}
func (stub *storeStub) AlbumLikes(_ context.Context, userID uuid.UUID, page PageRequest) (LikePage, error) {
	return stub.listed(userID, page)
}
func (stub *storeStub) LikeComment(_ context.Context, userID, id uuid.UUID) (Like, error) {
	return stub.liked(userID, id)
}
func (stub *storeStub) UnlikeComment(_ context.Context, userID, id uuid.UUID) error {
	return stub.unliked(userID, id)
}
func (stub *storeStub) CommentLikes(_ context.Context, userID uuid.UUID, page PageRequest) (LikePage, error) {
	return stub.listed(userID, page)
}

func (stub *storeStub) FollowArtist(_ context.Context, userID, artistID uuid.UUID) (Follow, error) {
	stub.userID, stub.targetID = userID, artistID
	stub.followCalls++
	return Follow{UserID: userID, ArtistID: artistID, FollowedAt: time.Now().UTC()}, nil
}

func (stub *storeStub) UnfollowArtist(_ context.Context, userID, artistID uuid.UUID) error {
	stub.userID, stub.targetID = userID, artistID
	stub.unfollowCalls++
	return nil
}

func (stub *storeStub) ArtistFollows(
	_ context.Context, userID uuid.UUID, page PageRequest,
) (FollowPage, error) {
	stub.userID, stub.page = userID, page
	return FollowPage{Items: []Follow{}, Limit: page.Limit, Offset: page.Offset}, nil
}
