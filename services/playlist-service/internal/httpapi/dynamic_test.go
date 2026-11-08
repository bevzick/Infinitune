package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/identity"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/rules"
	"github.com/google/uuid"
)

func TestCreateDynamicPlaylistUsesExplicitTypedCriteria(t *testing.T) {
	actorID := uuid.New()
	playlistID := uuid.New()
	ruleSetID := uuid.New()
	artistID := uuid.New()
	genreID := uuid.New()
	booleanValue := false
	service := &dynamicPlaylistsStub{result: playlist.DynamicPlaylist{
		Playlist: playlist.Playlist{
			ID: playlistID, OwnerID: actorID, Name: "Fresh discoveries",
			Type: playlist.TypeDynamic, Visibility: playlist.VisibilityPublic,
			RuleSetID: &ruleSetID,
		},
		RuleSet: rules.RuleSet{
			ID: ruleSetID, OwnerID: actorID, Name: "Recent clean tracks", Logic: rules.LogicAnd,
			Conditions: []rules.Condition{{
				ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldExplicit,
				Operator: rules.OperatorEquals, Value: rules.BooleanValue(booleanValue), Position: 1,
			}},
		},
	}}
	dependencies := testDependencies(databaseStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: actorID}}
	dependencies.DynamicPlaylists = service
	body := `{
		"name":"Fresh discoveries",
		"description":"Changes with the catalog",
		"visibility":"public",
		"criteria":{
			"name":"Recent clean tracks",
			"logic":"AND",
			"conditions":[
				{"field":"title","operator":"contains","value":{"kind":"text","text":"light"}},
				{"field":"artist_id","operator":"equals","value":{"kind":"uuid","uuid":"` + artistID.String() + `"}},
				{"field":"genre_id","operator":"in","value":{"kind":"uuid_list","uuid_list":["` + genreID.String() + `"]}},
				{"field":"release_year","operator":"greater_than_or_equal","value":{"kind":"integer","integer":2020}},
				{"field":"duration_ms","operator":"between","value":{"kind":"integer_range","integer_range":{"minimum":60000,"maximum":300000}}},
				{"field":"explicit","operator":"equals","value":{"kind":"boolean","boolean":false}}
			]
		}
	}`
	request := httptest.NewRequest(http.MethodPost, "/v1/playlists/dynamic", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusCreated || service.actorID != actorID ||
		service.createInput.Playlist.Visibility != playlist.VisibilityPublic ||
		len(service.createInput.Criteria.Conditions) != 6 {
		t.Fatalf("status = %d, actor = %s, input = %+v, body = %s",
			response.Code, service.actorID, service.createInput, response.Body.String())
	}
	conditions := service.createInput.Criteria.Conditions
	if conditions[0].Value.Kind != rules.ValueText || conditions[0].Value.Text != "light" ||
		conditions[1].Value.UUID != artistID || len(conditions[2].Value.UUIDs) != 1 ||
		conditions[2].Value.UUIDs[0] != genreID || conditions[3].Value.Integer != 2020 ||
		conditions[4].Value.IntegerRange.Minimum != 60000 ||
		conditions[4].Value.IntegerRange.Maximum != 300000 ||
		conditions[5].Value.Kind != rules.ValueBoolean || conditions[5].Value.Boolean {
		t.Fatalf("typed conditions = %+v", conditions)
	}
	if response.Header().Get("Location") != "/v1/playlists/"+playlistID.String() ||
		!strings.Contains(response.Body.String(), `"type":"dynamic"`) ||
		!strings.Contains(response.Body.String(), `"kind":"boolean","boolean":false`) {
		t.Fatalf("Location = %q, body = %s", response.Header().Get("Location"), response.Body.String())
	}
}

func TestDynamicCriteriaRoutesRequireOwnerAndPreserveTypedValue(t *testing.T) {
	actorID := uuid.New()
	playlistID := uuid.New()
	ruleSetID := uuid.New()
	service := &dynamicPlaylistsStub{result: playlist.DynamicPlaylist{RuleSet: rules.RuleSet{
		ID: ruleSetID, OwnerID: actorID, Name: "Current releases", Logic: rules.LogicAnd,
		Conditions: []rules.Condition{{
			ID: uuid.New(), RuleSetID: ruleSetID, Field: rules.FieldReleaseYear,
			Operator: rules.OperatorGreaterThanOrEqual, Value: rules.IntegerValue(2020), Position: 1,
		}},
	}}}
	dependencies := testDependencies(databaseStub{})
	dependencies.Tokens = identityVerifierStub{identity: identity.Identity{UserID: actorID}}
	dependencies.DynamicPlaylists = service
	router := NewRouter(dependencies)

	get := httptest.NewRequest(http.MethodGet,
		"/v1/playlists/"+playlistID.String()+"/criteria", nil)
	get.Header.Set("Authorization", "Bearer valid")
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK || service.actorID != actorID ||
		service.playlistID != playlistID ||
		!strings.Contains(getResponse.Body.String(), `"kind":"integer","integer":2020`) {
		t.Fatalf("GET status = %d, actor = %s, playlist = %s, body = %s",
			getResponse.Code, service.actorID, service.playlistID, getResponse.Body.String())
	}

	replace := httptest.NewRequest(http.MethodPut,
		"/v1/playlists/"+playlistID.String()+"/criteria",
		strings.NewReader(`{
			"name":"Only clean tracks","logic":"OR","conditions":[
				{"field":"explicit","operator":"equals","value":{"kind":"boolean","boolean":false}}
			]
		}`))
	replace.Header.Set("Authorization", "Bearer valid")
	replaceResponse := httptest.NewRecorder()
	router.ServeHTTP(replaceResponse, replace)
	if replaceResponse.Code != http.StatusOK || service.replaceInput.Logic != rules.LogicOr ||
		len(service.replaceInput.Conditions) != 1 ||
		service.replaceInput.Conditions[0].Value.Kind != rules.ValueBoolean ||
		service.replaceInput.Conditions[0].Value.Boolean {
		t.Fatalf("PUT status = %d, input = %+v, body = %s",
			replaceResponse.Code, service.replaceInput, replaceResponse.Body.String())
	}
}

func TestDynamicCriteriaRejectsAmbiguousValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "missing typed value", value: `{"kind":"boolean"}`},
		{name: "mismatched typed value", value: `{"kind":"boolean","integer":1}`},
		{name: "multiple typed values", value: `{"kind":"integer","integer":1,"boolean":false}`},
		{name: "unknown kind", value: `{"kind":"decimal","integer":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &dynamicPlaylistsStub{}
			dependencies := testDependencies(databaseStub{})
			dependencies.DynamicPlaylists = service
			request := httptest.NewRequest(http.MethodPost, "/v1/playlists/dynamic",
				strings.NewReader(`{
					"name":"Invalid","criteria":{"name":"Invalid","logic":"AND","conditions":[
						{"field":"explicit","operator":"equals","value":`+test.value+`}
					]}
				}`))
			request.Header.Set("Authorization", "Bearer valid")
			response := httptest.NewRecorder()
			NewRouter(dependencies).ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity || service.createCalls != 0 ||
				!strings.Contains(response.Body.String(), `"code":"invalid_criteria"`) {
				t.Fatalf("status = %d, calls = %d, body = %s",
					response.Code, service.createCalls, response.Body.String())
			}
		})
	}
}

func TestDynamicCriteriaRoutesRequireIdentity(t *testing.T) {
	dependencies := testDependencies(databaseStub{})
	request := httptest.NewRequest(http.MethodGet,
		"/v1/playlists/"+uuid.NewString()+"/criteria", nil)
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized ||
		response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("status = %d, authenticate = %q, body = %s",
			response.Code, response.Header().Get("WWW-Authenticate"), response.Body.String())
	}
}

func TestCreateDynamicPlaylistMapsRuleValidationError(t *testing.T) {
	service := &dynamicPlaylistsStub{err: rules.ErrInvalid}
	dependencies := testDependencies(databaseStub{})
	dependencies.DynamicPlaylists = service
	request := httptest.NewRequest(http.MethodPost, "/v1/playlists/dynamic", strings.NewReader(`{
		"name":"Invalid rules",
		"criteria":{"name":"Invalid rules","logic":"AND","conditions":[
			{"field":"explicit","operator":"equals","value":{"kind":"boolean","boolean":false}}
		]}
	}`))
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity || service.createCalls != 1 ||
		!strings.Contains(response.Body.String(), `"code":"invalid_criteria"`) {
		t.Fatalf("status = %d, calls = %d, body = %s",
			response.Code, service.createCalls, response.Body.String())
	}
}

func TestResolveDynamicTracksDescribesReadyAndEmptyResults(t *testing.T) {
	playlistID := uuid.New()
	trackID := uuid.New()
	tests := []struct {
		name     string
		page     playlist.DynamicTrackPage
		contains []string
	}{
		{
			name: "ready",
			page: playlist.DynamicTrackPage{
				Items: []playlist.DynamicTrack{{TrackID: trackID, Position: 3}},
				Limit: 1, Offset: 2, Total: 4, Status: playlist.DynamicResolutionReady,
			},
			contains: []string{`"status":"ready"`, `"track_id":"` + trackID.String() + `"`, `"position":3`},
		},
		{
			name: "empty",
			page: playlist.DynamicTrackPage{
				Items: []playlist.DynamicTrack{}, Limit: 1, Offset: 2,
				Status: playlist.DynamicResolutionEmpty,
			},
			contains: []string{`"items":[]`, `"status":"empty"`, `"reason":"no_matches"`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &dynamicTracksStub{result: test.page}
			dependencies := testDependencies(databaseStub{})
			dependencies.DynamicTracks = service
			request := httptest.NewRequest(http.MethodGet,
				"/v1/playlists/"+playlistID.String()+"/resolved-tracks?limit=1&offset=2", nil)
			response := httptest.NewRecorder()
			NewRouter(dependencies).ServeHTTP(response, request)
			if response.Code != http.StatusOK || service.viewerID != uuid.Nil ||
				service.playlistID != playlistID ||
				service.page != (playlist.PageRequest{Limit: 1, Offset: 2}) {
				t.Fatalf("status = %d, viewer = %s, playlist = %s, page = %+v, body = %s",
					response.Code, service.viewerID, service.playlistID, service.page, response.Body.String())
			}
			for _, expected := range test.contains {
				if !strings.Contains(response.Body.String(), expected) {
					t.Fatalf("body = %s, want %s", response.Body.String(), expected)
				}
			}
		})
	}
}

func TestResolveDynamicTracksDistinguishesCatalogFailures(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "timeout", err: playlist.ErrCatalogTimeout,
			wantStatus: http.StatusGatewayTimeout, wantCode: "catalog_timeout"},
		{name: "unavailable", err: playlist.ErrCatalogUnavailable,
			wantStatus: http.StatusServiceUnavailable, wantCode: "catalog_unavailable"},
		{name: "manual playlist", err: playlist.ErrDynamicOnly,
			wantStatus: http.StatusConflict, wantCode: "dynamic_playlist_required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dependencies := testDependencies(databaseStub{})
			dependencies.DynamicTracks = &dynamicTracksStub{err: test.err}
			request := httptest.NewRequest(http.MethodGet,
				"/v1/playlists/"+uuid.NewString()+"/resolved-tracks", nil)
			response := httptest.NewRecorder()
			NewRouter(dependencies).ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantCode) {
				t.Fatalf("status = %d, body = %s; want %d and %q",
					response.Code, response.Body.String(), test.wantStatus, test.wantCode)
			}
		})
	}
}

type dynamicPlaylistsStub struct {
	result       playlist.DynamicPlaylist
	err          error
	actorID      uuid.UUID
	playlistID   uuid.UUID
	createInput  playlist.DynamicCreateInput
	replaceInput rules.CreateInput
	createCalls  int
}

func (stub *dynamicPlaylistsStub) Create(
	_ context.Context,
	actorID uuid.UUID,
	input playlist.DynamicCreateInput,
) (playlist.DynamicPlaylist, error) {
	stub.actorID = actorID
	stub.createInput = input
	stub.createCalls++
	return stub.result, stub.err
}

func (stub *dynamicPlaylistsStub) Criteria(
	_ context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
) (rules.RuleSet, error) {
	stub.actorID = actorID
	stub.playlistID = playlistID
	return stub.result.RuleSet, stub.err
}

func (stub *dynamicPlaylistsStub) ReplaceCriteria(
	_ context.Context,
	actorID uuid.UUID,
	playlistID uuid.UUID,
	input rules.CreateInput,
) (rules.RuleSet, error) {
	stub.actorID = actorID
	stub.playlistID = playlistID
	stub.replaceInput = input
	return stub.result.RuleSet, stub.err
}

type dynamicTracksStub struct {
	result     playlist.DynamicTrackPage
	err        error
	viewerID   uuid.UUID
	playlistID uuid.UUID
	page       playlist.PageRequest
}

func (stub *dynamicTracksStub) Resolve(
	_ context.Context,
	viewerID uuid.UUID,
	playlistID uuid.UUID,
	page playlist.PageRequest,
) (playlist.DynamicTrackPage, error) {
	stub.viewerID = viewerID
	stub.playlistID = playlistID
	stub.page = page
	return stub.result, stub.err
}

var (
	_ DynamicPlaylists = (*dynamicPlaylistsStub)(nil)
	_ DynamicTracks    = (*dynamicTracksStub)(nil)
)
