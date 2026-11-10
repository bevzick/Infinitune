package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/identity"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
)

func TestLikeEndpointsUseOnlyVerifiedIdentity(t *testing.T) {
	userID, trackID := uuid.New(), uuid.New()
	likes := &likesStub{}
	router := NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}},
		Likes: likes,
	})
	request := httptest.NewRequest(http.MethodPut,
		"/v1/interactions/tracks/"+trackID.String()+"/like", nil)
	request.Header.Set("Authorization", "Bearer verified-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if likes.userID != userID || likes.targetID != trackID {
		t.Fatalf("like owner = %s, target = %s", likes.userID, likes.targetID)
	}
}

func TestLikeEndpointsRejectMissingTokenBeforeStorage(t *testing.T) {
	likes := &likesStub{}
	router := NewRouter(Dependencies{Database: databaseStub{}, Tokens: tokenStub{}, Likes: likes})
	request := httptest.NewRequest(http.MethodDelete,
		"/v1/interactions/albums/"+uuid.NewString()+"/like", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || likes.calls != 0 {
		t.Fatalf("status = %d, storage calls = %d", response.Code, likes.calls)
	}
}

func TestCommentLikeEndpointsUseVerifiedIdentityAndMapUnavailableTarget(t *testing.T) {
	userID, commentID := uuid.New(), uuid.New()
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "visible", want: http.StatusOK},
		{name: "unavailable", err: interaction.ErrCommentNotFound, want: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			likes := &likesStub{err: test.err}
			router := NewRouter(Dependencies{
				Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}}, Likes: likes,
			})
			request := httptest.NewRequest(http.MethodPut,
				"/v1/interactions/comments/"+commentID.String()+"/like", nil)
			request.Header.Set("Authorization", "Bearer verified-token")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want || likes.userID != userID || likes.targetID != commentID {
				t.Fatalf("status/owner/target = %d/%s/%s; body = %s",
					response.Code, likes.userID, likes.targetID, response.Body.String())
			}
		})
	}
}

func TestLikeEndpointsMapCatalogAndPaginationErrors(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name       string
		path       string
		likes      *likesStub
		wantStatus int
	}{
		{name: "missing track", path: "/v1/interactions/tracks/" + uuid.NewString() + "/like",
			likes: &likesStub{err: interaction.ErrTrackNotFound}, wantStatus: http.StatusNotFound},
		{name: "invalid page", path: "/v1/interactions/tracks/likes?limit=invalid",
			likes: &likesStub{}, wantStatus: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := NewRouter(Dependencies{
				Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}},
				Likes: test.likes,
			})
			method := http.MethodGet
			if test.name == "missing track" {
				method = http.MethodPut
			}
			request := httptest.NewRequest(method, test.path, nil)
			request.Header.Set("Authorization", "Bearer verified-token")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s",
					response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

type tokenStub struct {
	identity identity.Identity
	err      error
}

func (stub tokenStub) Verify(context.Context, string, time.Time) (identity.Identity, error) {
	if stub.err != nil {
		return identity.Identity{}, stub.err
	}
	if stub.identity.UserID == uuid.Nil {
		return identity.Identity{}, errors.New("identity unavailable")
	}
	return stub.identity, nil
}

type likesStub struct {
	userID   uuid.UUID
	targetID uuid.UUID
	err      error
	calls    int
}

func (stub *likesStub) like(userID, targetID uuid.UUID) (interaction.Like, error) {
	stub.userID, stub.targetID = userID, targetID
	stub.calls++
	return interaction.Like{UserID: userID, TargetID: targetID, CreatedAt: time.Now().UTC()}, stub.err
}

func (stub *likesStub) unlike(userID, targetID uuid.UUID) error {
	stub.userID, stub.targetID = userID, targetID
	stub.calls++
	return stub.err
}

func (stub *likesStub) list(userID uuid.UUID, page interaction.PageRequest) (interaction.LikePage, error) {
	stub.userID = userID
	stub.calls++
	return interaction.LikePage{Items: []interaction.Like{}, Limit: page.Limit, Offset: page.Offset}, stub.err
}

func (stub *likesStub) LikeTrack(_ context.Context, userID, id uuid.UUID) (interaction.Like, error) {
	return stub.like(userID, id)
}
func (stub *likesStub) UnlikeTrack(_ context.Context, userID, id uuid.UUID) error {
	return stub.unlike(userID, id)
}
func (stub *likesStub) TrackLikes(_ context.Context, userID uuid.UUID, page interaction.PageRequest) (interaction.LikePage, error) {
	return stub.list(userID, page)
}
func (stub *likesStub) LikeAlbum(_ context.Context, userID, id uuid.UUID) (interaction.Like, error) {
	return stub.like(userID, id)
}
func (stub *likesStub) UnlikeAlbum(_ context.Context, userID, id uuid.UUID) error {
	return stub.unlike(userID, id)
}
func (stub *likesStub) AlbumLikes(_ context.Context, userID uuid.UUID, page interaction.PageRequest) (interaction.LikePage, error) {
	return stub.list(userID, page)
}
func (stub *likesStub) LikeComment(_ context.Context, userID, id uuid.UUID) (interaction.Like, error) {
	return stub.like(userID, id)
}
func (stub *likesStub) UnlikeComment(_ context.Context, userID, id uuid.UUID) error {
	return stub.unlike(userID, id)
}
func (stub *likesStub) CommentLikes(_ context.Context, userID uuid.UUID, page interaction.PageRequest) (interaction.LikePage, error) {
	return stub.list(userID, page)
}
