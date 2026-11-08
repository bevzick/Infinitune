package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/identity"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/google/uuid"
)

type databaseStub struct{ err error }

func (stub databaseStub) Ping(context.Context) error { return stub.err }

func TestHealthReflectsDatabaseAvailability(t *testing.T) {
	tests := []struct {
		name       string
		database   DatabaseCheck
		wantStatus int
	}{
		{name: "available", database: databaseStub{}, wantStatus: http.StatusOK},
		{name: "unavailable", database: databaseStub{err: errors.New("database")},
			wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()
			NewRouter(testDependencies(test.database)).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s",
					response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestMutationsRequireVerifiedIdentity(t *testing.T) {
	dependencies := testDependencies(databaseStub{})
	dependencies.Tokens = identityVerifierStub{err: identity.ErrInvalidAccessToken}
	request := httptest.NewRequest(http.MethodPost, "/v1/playlists",
		strings.NewReader(`{"name":"Rejected"}`))
	request.Header.Set("Authorization", "Bearer invalid")
	response := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status = %d, authenticate = %q, body = %s",
			response.Code, response.Header().Get("WWW-Authenticate"), response.Body.String())
	}
}

func TestCreateUsesTokenOwnerAndRejectsBodyOwner(t *testing.T) {
	actorID := uuid.New()
	playlists := &playlistsStub{result: playlist.Playlist{
		ID: uuid.New(), OwnerID: actorID, Name: "Road trip", Type: playlist.TypeManual,
		Visibility: playlist.VisibilityPrivate,
	}}
	dependencies := testDependencies(databaseStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: actorID}}
	dependencies.Playlists = playlists
	router := NewRouter(dependencies)

	spoofed := httptest.NewRequest(http.MethodPost, "/v1/playlists",
		strings.NewReader(`{"name":"Road trip","owner_id":"`+uuid.NewString()+`"}`))
	spoofed.Header.Set("Authorization", "Bearer valid")
	spoofedResponse := httptest.NewRecorder()
	router.ServeHTTP(spoofedResponse, spoofed)
	if spoofedResponse.Code != http.StatusBadRequest || playlists.createCalls != 0 {
		t.Fatalf("spoofed create status = %d, calls = %d, body = %s",
			spoofedResponse.Code, playlists.createCalls, spoofedResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/playlists",
		strings.NewReader(`{"name":"Road trip","visibility":"private"}`))
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || playlists.actorID != actorID || playlists.createCalls != 1 {
		t.Fatalf("status = %d, actor = %s, calls = %d, body = %s",
			response.Code, playlists.actorID, playlists.createCalls, response.Body.String())
	}
	if response.Header().Get("Location") != "/v1/playlists/"+playlists.result.ID.String() {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
}

func TestReadsSupportAnonymousAndVerifiedViewer(t *testing.T) {
	playlistID := uuid.New()
	viewerID := uuid.New()
	playlists := &playlistsStub{result: playlist.Playlist{
		ID: playlistID, OwnerID: viewerID, Name: "Visible", Type: playlist.TypeManual,
		Visibility: playlist.VisibilityPublic,
	}}
	dependencies := testDependencies(databaseStub{})
	dependencies.Playlists = playlists
	router := NewRouter(dependencies)

	anonymous := httptest.NewRequest(http.MethodGet, "/v1/playlists/"+playlistID.String(), nil)
	anonymousResponse := httptest.NewRecorder()
	router.ServeHTTP(anonymousResponse, anonymous)
	if anonymousResponse.Code != http.StatusOK || playlists.viewerID != uuid.Nil {
		t.Fatalf("anonymous status = %d, viewer = %s", anonymousResponse.Code, playlists.viewerID)
	}

	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: viewerID}}
	router = NewRouter(dependencies)
	verified := httptest.NewRequest(http.MethodGet, "/v1/playlists/"+playlistID.String(), nil)
	verified.Header.Set("Authorization", "Bearer valid")
	verifiedResponse := httptest.NewRecorder()
	router.ServeHTTP(verifiedResponse, verified)
	if verifiedResponse.Code != http.StatusOK || playlists.viewerID != viewerID {
		t.Fatalf("verified status = %d, viewer = %s", verifiedResponse.Code, playlists.viewerID)
	}

	dependencies.Tokens = identityVerifierStub{err: identity.ErrInvalidAccessToken}
	invalid := httptest.NewRequest(http.MethodGet, "/v1/playlists/"+playlistID.String(), nil)
	invalid.Header.Set("Authorization", "Bearer invalid")
	invalidResponse := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusUnauthorized {
		t.Fatalf("invalid optional token status = %d, body = %s",
			invalidResponse.Code, invalidResponse.Body.String())
	}
}

func TestPlaylistMutationRoutesPassVerifiedActor(t *testing.T) {
	actorID := uuid.New()
	playlistID := uuid.New()
	playlists := &playlistsStub{result: playlist.Playlist{
		ID: playlistID, OwnerID: actorID, Name: "Updated", Type: playlist.TypeManual,
		Visibility: playlist.VisibilityUnlisted,
	}}
	dependencies := testDependencies(databaseStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: actorID}}
	dependencies.Playlists = playlists
	router := NewRouter(dependencies)

	patch := httptest.NewRequest(http.MethodPatch, "/v1/playlists/"+playlistID.String(),
		strings.NewReader(`{"name":"Updated","visibility":"unlisted"}`))
	patch.Header.Set("Authorization", "Bearer valid")
	patchResponse := httptest.NewRecorder()
	router.ServeHTTP(patchResponse, patch)
	if patchResponse.Code != http.StatusOK || playlists.actorID != actorID ||
		playlists.updateInput.Name == nil || *playlists.updateInput.Name != "Updated" ||
		playlists.updateInput.Visibility == nil ||
		*playlists.updateInput.Visibility != playlist.VisibilityUnlisted {
		t.Fatalf("patch status = %d, actor = %s, input = %+v, body = %s",
			patchResponse.Code, playlists.actorID, playlists.updateInput, patchResponse.Body.String())
	}

	requestDelete := httptest.NewRequest(http.MethodDelete, "/v1/playlists/"+playlistID.String(), nil)
	requestDelete.Header.Set("Authorization", "Bearer valid")
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, requestDelete)
	if deleteResponse.Code != http.StatusNoContent || playlists.actorID != actorID ||
		playlists.playlistID != playlistID {
		t.Fatalf("delete status = %d, actor = %s, playlist = %s",
			deleteResponse.Code, playlists.actorID, playlists.playlistID)
	}
}

func TestTrackRoutesPassVerifiedActorAndOrder(t *testing.T) {
	actorID := uuid.New()
	playlistID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()
	tracks := &tracksStub{result: []playlist.Track{
		{PlaylistID: playlistID, TrackID: secondID, Position: 1, AddedBy: actorID},
		{PlaylistID: playlistID, TrackID: firstID, Position: 2, AddedBy: actorID},
	}}
	dependencies := testDependencies(databaseStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: actorID}}
	dependencies.Tracks = tracks
	router := NewRouter(dependencies)
	get := httptest.NewRequest(http.MethodGet,
		"/v1/playlists/"+playlistID.String()+"/tracks?limit=1&offset=1", nil)
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK || tracks.page.Limit != 1 || tracks.page.Offset != 1 ||
		!strings.Contains(getResponse.Body.String(), `"total":2`) {
		t.Fatalf("get status = %d, page = %+v, body = %s",
			getResponse.Code, tracks.page, getResponse.Body.String())
	}

	add := httptest.NewRequest(http.MethodPost, "/v1/playlists/"+playlistID.String()+"/tracks",
		strings.NewReader(`{"track_id":"`+firstID.String()+`"}`))
	add.Header.Set("Authorization", "Bearer valid")
	addResponse := httptest.NewRecorder()
	router.ServeHTTP(addResponse, add)
	if addResponse.Code != http.StatusCreated || tracks.actorID != actorID || tracks.trackID != firstID {
		t.Fatalf("add status = %d, actor = %s, track = %s, body = %s",
			addResponse.Code, tracks.actorID, tracks.trackID, addResponse.Body.String())
	}

	reorder := httptest.NewRequest(http.MethodPut, "/v1/playlists/"+playlistID.String()+"/tracks/order",
		strings.NewReader(`{"track_ids":["`+secondID.String()+`","`+firstID.String()+`"]}`))
	reorder.Header.Set("Authorization", "Bearer valid")
	reorderResponse := httptest.NewRecorder()
	router.ServeHTTP(reorderResponse, reorder)
	if reorderResponse.Code != http.StatusOK || len(tracks.order) != 2 ||
		tracks.order[0] != secondID || tracks.order[1] != firstID {
		t.Fatalf("reorder status = %d, order = %v, body = %s",
			reorderResponse.Code, tracks.order, reorderResponse.Body.String())
	}

	remove := httptest.NewRequest(http.MethodDelete,
		"/v1/playlists/"+playlistID.String()+"/tracks/"+firstID.String(), nil)
	remove.Header.Set("Authorization", "Bearer valid")
	removeResponse := httptest.NewRecorder()
	router.ServeHTTP(removeResponse, remove)
	if removeResponse.Code != http.StatusNoContent || tracks.trackID != firstID {
		t.Fatalf("remove status = %d, track = %s", removeResponse.Code, tracks.trackID)
	}
}

func TestDomainErrorsUseStableHTTPResponses(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "missing", err: playlist.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "playlist_not_found"},
		{name: "forbidden", err: playlist.ErrForbidden, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "catalog unavailable", err: playlist.ErrCatalogUnavailable,
			wantStatus: http.StatusServiceUnavailable, wantCode: "catalog_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dependencies := testDependencies(databaseStub{})
			dependencies.Playlists = &playlistsStub{err: test.err}
			request := httptest.NewRequest(http.MethodGet, "/v1/playlists/"+uuid.NewString(), nil)
			response := httptest.NewRecorder()
			NewRouter(dependencies).ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantCode) {
				t.Fatalf("status = %d, body = %s; want %d and %q",
					response.Code, response.Body.String(), test.wantStatus, test.wantCode)
			}
		})
	}
}

func TestCORSPreflightDoesNotRequireToken(t *testing.T) {
	dependencies := testDependencies(databaseStub{})
	dependencies.Origins = []string{"http://localhost:5173"}
	request := httptest.NewRequest(http.MethodOptions, "/v1/playlists", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	response := httptest.NewRecorder()
	NewRouter(dependencies).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent ||
		response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("status = %d, headers = %v", response.Code, response.Header())
	}
}

func testDependencies(database DatabaseCheck) Dependencies {
	return Dependencies{
		Database:  database,
		Tokens:    identityVerifierStub{identity: identity.Identity{UserID: uuid.New()}},
		Playlists: &playlistsStub{}, Tracks: &tracksStub{},
		DynamicPlaylists: &dynamicPlaylistsStub{}, DynamicTracks: &dynamicTracksStub{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

type identityVerifierStub struct {
	identity identity.Identity
	err      error
}

func (stub identityVerifierStub) Verify(context.Context, string, time.Time) (identity.Identity, error) {
	return stub.identity, stub.err
}

type playlistsStub struct {
	result      playlist.Playlist
	err         error
	actorID     uuid.UUID
	viewerID    uuid.UUID
	playlistID  uuid.UUID
	createInput playlist.CreateInput
	updateInput playlist.UpdateInput
	createCalls int
}

func (stub *playlistsStub) Create(
	_ context.Context,
	actorID uuid.UUID,
	input playlist.CreateInput,
) (playlist.Playlist, error) {
	stub.actorID = actorID
	stub.createInput = input
	stub.createCalls++
	return stub.result, stub.err
}

func (stub *playlistsStub) ByID(
	_ context.Context,
	viewerID uuid.UUID,
	playlistID uuid.UUID,
) (playlist.Playlist, error) {
	stub.viewerID = viewerID
	stub.playlistID = playlistID
	return stub.result, stub.err
}

func (stub *playlistsStub) Update(
	_ context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
	input playlist.UpdateInput,
) (playlist.Playlist, error) {
	stub.actorID = actorID
	stub.playlistID = playlistID
	stub.updateInput = input
	return stub.result, stub.err
}

func (stub *playlistsStub) Delete(_ context.Context, actorID, playlistID uuid.UUID) error {
	stub.actorID = actorID
	stub.playlistID = playlistID
	return stub.err
}

type tracksStub struct {
	result     []playlist.Track
	err        error
	actorID    uuid.UUID
	viewerID   uuid.UUID
	playlistID uuid.UUID
	trackID    uuid.UUID
	order      []uuid.UUID
	page       playlist.PageRequest
}

func (stub *tracksStub) Tracks(
	_ context.Context,
	viewerID uuid.UUID,
	playlistID uuid.UUID,
	page playlist.PageRequest,
) (playlist.TrackPage, error) {
	stub.viewerID = viewerID
	stub.playlistID = playlistID
	stub.page = page
	if page.Limit == 0 {
		page.Limit = 50
	}
	return playlist.TrackPage{
		Items: stub.result, Limit: page.Limit, Offset: page.Offset, Total: int64(len(stub.result)),
	}, stub.err
}

func (stub *tracksStub) Add(
	_ context.Context,
	actorID, playlistID, trackID uuid.UUID,
) (playlist.Track, error) {
	stub.actorID = actorID
	stub.playlistID = playlistID
	stub.trackID = trackID
	if len(stub.result) == 0 {
		return playlist.Track{PlaylistID: playlistID, TrackID: trackID, AddedBy: actorID}, stub.err
	}
	return stub.result[0], stub.err
}

func (stub *tracksStub) Remove(
	_ context.Context,
	actorID, playlistID, trackID uuid.UUID,
) error {
	stub.actorID = actorID
	stub.playlistID = playlistID
	stub.trackID = trackID
	return stub.err
}

func (stub *tracksStub) Reorder(
	_ context.Context,
	actorID, playlistID uuid.UUID,
	trackIDs []uuid.UUID,
) ([]playlist.Track, error) {
	stub.actorID = actorID
	stub.playlistID = playlistID
	stub.order = append([]uuid.UUID(nil), trackIDs...)
	return stub.result, stub.err
}
