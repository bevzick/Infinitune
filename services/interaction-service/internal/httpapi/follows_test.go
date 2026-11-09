package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/interaction-service/internal/identity"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
)

func TestFollowEndpointsUseVerifiedIdentity(t *testing.T) {
	userID, artistID := uuid.New(), uuid.New()
	follows := &followsStub{}
	router := NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}},
		Follows: follows,
	})
	request := httptest.NewRequest(http.MethodPut,
		"/v1/interactions/artists/"+artistID.String()+"/follow", nil)
	request.Header.Set("Authorization", "Bearer verified-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if follows.userID != userID || follows.artistID != artistID {
		t.Fatalf("follow owner = %s, artist = %s", follows.userID, follows.artistID)
	}
}

func TestFollowEndpointsMapMissingArtistAndListOwnFollows(t *testing.T) {
	userID := uuid.New()
	missing := &followsStub{err: interaction.ErrArtistNotFound}
	router := NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}},
		Follows: missing,
	})
	request := httptest.NewRequest(http.MethodPut,
		"/v1/interactions/artists/"+uuid.NewString()+"/follow", nil)
	request.Header.Set("Authorization", "Bearer verified-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing artist status = %d; body = %s", response.Code, response.Body.String())
	}

	follows := &followsStub{}
	router = NewRouter(Dependencies{
		Database: databaseStub{}, Tokens: tokenStub{identity: identity.Identity{UserID: userID}},
		Follows: follows,
	})
	request = httptest.NewRequest(http.MethodGet, "/v1/interactions/artists/follows?limit=10", nil)
	request.Header.Set("Authorization", "Bearer verified-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || follows.userID != userID || follows.page.Limit != 10 {
		t.Fatalf("list status = %d, owner = %s, page = %+v; body = %s",
			response.Code, follows.userID, follows.page, response.Body.String())
	}
}

type followsStub struct {
	userID   uuid.UUID
	artistID uuid.UUID
	page     interaction.PageRequest
	err      error
}

func (stub *followsStub) FollowArtist(
	_ context.Context, userID, artistID uuid.UUID,
) (interaction.Follow, error) {
	stub.userID, stub.artistID = userID, artistID
	return interaction.Follow{
		UserID: userID, ArtistID: artistID, FollowedAt: time.Now().UTC(),
	}, stub.err
}

func (stub *followsStub) UnfollowArtist(_ context.Context, userID, artistID uuid.UUID) error {
	stub.userID, stub.artistID = userID, artistID
	return stub.err
}

func (stub *followsStub) ArtistFollows(
	_ context.Context, userID uuid.UUID, page interaction.PageRequest,
) (interaction.FollowPage, error) {
	stub.userID, stub.page = userID, page
	return interaction.FollowPage{
		Items: []interaction.Follow{}, Limit: page.Limit, Offset: page.Offset,
	}, stub.err
}
