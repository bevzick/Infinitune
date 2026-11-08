package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
)

type healthCheckStub struct {
	err error
}

func (stub healthCheckStub) Ping(context.Context) error {
	return stub.err
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		database   HealthCheck
		wantStatus int
		wantBody   string
	}{
		{
			name:       "database available",
			database:   healthCheckStub{},
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "database unavailable",
			database:   healthCheckStub{err: errors.New("database unavailable")},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"status":"unavailable"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			response := httptest.NewRecorder()

			NewRouter(testDependencies(&catalogStub{}, test.database)).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Body.String() != test.wantBody {
				t.Errorf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
		})
	}
}

func TestBrowseRoutesUsePaginationAndReturnArrays(t *testing.T) {
	stub := &catalogStub{}
	router := NewRouter(testDependencies(stub, healthCheckStub{}))
	paths := []string{"artists", "albums", "tracks", "genres", "moods"}

	for _, resource := range paths {
		t.Run(resource, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				"/v1/catalog/"+resource+"?limit=7&offset=14",
				nil,
			)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			actualPage := stub.lastPage
			if resource == "tracks" {
				actualPage = stub.lastTrackQuery.Page
			}
			if actualPage != (metadata.PageRequest{Limit: 7, Offset: 14}) {
				t.Fatalf("page = %+v", actualPage)
			}
			var body struct {
				Items      []json.RawMessage  `json:"items"`
				Pagination paginationResponse `json:"pagination"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Items == nil || len(body.Items) != 0 {
				t.Fatalf("items = %#v, want empty array", body.Items)
			}
			if body.Pagination.Limit != 7 || body.Pagination.Offset != 14 {
				t.Fatalf("pagination = %+v", body.Pagination)
			}
		})
	}
}

func TestTrackDetailReturnsPublicMetadataWithoutInternalIDs(t *testing.T) {
	trackID := uuid.New()
	artistID := uuid.New()
	stub := &catalogStub{track: metadata.Track{
		ID: trackID, Title: "Public Track", DurationMS: 180000, TrackNumber: 1, DiscNumber: 1,
		ReleaseDate:  time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
		MediaAssetID: uuid.New(), ContentID: uuid.New(),
		Artists: []metadata.TrackArtist{{
			Artist: metadata.Artist{ID: artistID, Name: "Artist"},
			Role:   metadata.ArtistRolePrimary, Position: 1,
		}},
		Genres: []metadata.Genre{{ID: uuid.New(), Name: "Electronic", Slug: "electronic"}},
		Moods: []metadata.TrackMood{{
			Mood: metadata.Mood{ID: uuid.New(), Name: "Focused", Slug: "focused"}, Weight: 0.8,
		}},
	}}
	request := httptest.NewRequest(http.MethodGet, "/v1/catalog/tracks/"+trackID.String(), nil)
	response := httptest.NewRecorder()

	NewRouter(testDependencies(stub, healthCheckStub{})).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, field := range []string{"content_id", "media_asset_id"} {
		if _, exists := body[field]; exists {
			t.Errorf("public response exposes %s: %s", field, response.Body.String())
		}
	}
	if body["release_date"] != "2026-11-08" {
		t.Errorf("release_date = %v", body["release_date"])
	}
	if !strings.Contains(response.Body.String(), `"role":"primary"`) {
		t.Errorf("body does not contain artist role: %s", response.Body.String())
	}
}

func TestCatalogRoutesReturnStructuredClientErrors(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		stub       *catalogStub
		wantStatus int
		wantCode   string
	}{
		{
			name: "invalid id", path: "/v1/catalog/tracks/not-a-uuid",
			stub: &catalogStub{}, wantStatus: http.StatusBadRequest, wantCode: "invalid_id",
		},
		{
			name: "invalid limit", path: "/v1/catalog/tracks?limit=101",
			stub: &catalogStub{}, wantStatus: http.StatusBadRequest, wantCode: "invalid_pagination",
		},
		{
			name: "negative offset", path: "/v1/catalog/tracks?offset=-1",
			stub: &catalogStub{}, wantStatus: http.StatusBadRequest, wantCode: "invalid_pagination",
		},
		{
			name: "invalid artist filter", path: "/v1/catalog/tracks?artist_id=invalid",
			stub: &catalogStub{}, wantStatus: http.StatusBadRequest, wantCode: "invalid_filter",
		},
		{
			name: "invalid explicit filter", path: "/v1/catalog/tracks?explicit=yes",
			stub: &catalogStub{}, wantStatus: http.StatusBadRequest, wantCode: "invalid_filter",
		},
		{
			name: "invalid sort", path: "/v1/catalog/tracks?sort=random",
			stub: &catalogStub{}, wantStatus: http.StatusBadRequest, wantCode: "invalid_filter",
		},
		{
			name: "not found", path: "/v1/catalog/tracks/" + uuid.NewString(),
			stub:       &catalogStub{err: metadata.ErrNotFound},
			wantStatus: http.StatusNotFound, wantCode: "catalog_item_not_found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			NewRouter(testDependencies(test.stub, healthCheckStub{})).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			var body errorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Error.Code != test.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, test.wantCode)
			}
		})
	}
}

func TestTrackSearchPassesCombinedFilters(t *testing.T) {
	artistID := uuid.New()
	albumID := uuid.New()
	genreID := uuid.New()
	moodID := uuid.New()
	stub := &catalogStub{}
	path := "/v1/catalog/tracks?title=night&artist_id=" + artistID.String() +
		"&album_id=" + albumID.String() + "&genre_id=" + genreID.String() +
		"&mood_id=" + moodID.String() + "&release_year=2026&explicit=false&sort=title_asc"
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()

	NewRouter(testDependencies(stub, healthCheckStub{})).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	query := stub.lastTrackQuery
	if query.Title != "night" || query.ArtistID == nil || *query.ArtistID != artistID ||
		query.AlbumID == nil || *query.AlbumID != albumID ||
		query.GenreID == nil || *query.GenreID != genreID ||
		query.MoodID == nil || *query.MoodID != moodID ||
		query.ReleaseYear == nil || *query.ReleaseYear != 2026 ||
		query.Explicit == nil || *query.Explicit || query.Sort != metadata.TrackSortTitleAsc {
		t.Fatalf("track query = %+v", query)
	}
}

func TestCatalogCORSPreflight(t *testing.T) {
	dependencies := testDependencies(&catalogStub{}, healthCheckStub{})
	dependencies.Origins = []string{"http://localhost:5173"}
	request := httptest.NewRequest(http.MethodOptions, "/v1/catalog/tracks", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	response := httptest.NewRecorder()

	NewRouter(dependencies).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func testDependencies(catalog Catalog, database HealthCheck) Dependencies {
	return Dependencies{Database: database, Catalog: catalog}
}

type catalogStub struct {
	err            error
	track          metadata.Track
	lastPage       metadata.PageRequest
	lastTrackQuery metadata.TrackQuery
}

func (stub *catalogStub) Artist(context.Context, uuid.UUID) (metadata.Artist, error) {
	return metadata.Artist{}, stub.err
}

func (stub *catalogStub) Artists(_ context.Context, page metadata.PageRequest) (metadata.Page[metadata.Artist], error) {
	stub.lastPage = page
	return metadata.Page[metadata.Artist]{Items: []metadata.Artist{}, Limit: page.Limit, Offset: page.Offset}, stub.err
}

func (stub *catalogStub) Album(context.Context, uuid.UUID) (metadata.Album, error) {
	return metadata.Album{}, stub.err
}

func (stub *catalogStub) Albums(_ context.Context, page metadata.PageRequest) (metadata.Page[metadata.Album], error) {
	stub.lastPage = page
	return metadata.Page[metadata.Album]{Items: []metadata.Album{}, Limit: page.Limit, Offset: page.Offset}, stub.err
}

func (stub *catalogStub) Track(context.Context, uuid.UUID) (metadata.Track, error) {
	return stub.track, stub.err
}

func (stub *catalogStub) Tracks(_ context.Context, query metadata.TrackQuery) (metadata.Page[metadata.Track], error) {
	stub.lastTrackQuery = query
	return metadata.Page[metadata.Track]{
		Items: []metadata.Track{}, Limit: query.Page.Limit, Offset: query.Page.Offset,
	}, stub.err
}

func (stub *catalogStub) Genre(context.Context, uuid.UUID) (metadata.Genre, error) {
	return metadata.Genre{}, stub.err
}

func (stub *catalogStub) Genres(_ context.Context, page metadata.PageRequest) (metadata.Page[metadata.Genre], error) {
	stub.lastPage = page
	return metadata.Page[metadata.Genre]{Items: []metadata.Genre{}, Limit: page.Limit, Offset: page.Offset}, stub.err
}

func (stub *catalogStub) Mood(context.Context, uuid.UUID) (metadata.Mood, error) {
	return metadata.Mood{}, stub.err
}

func (stub *catalogStub) Moods(_ context.Context, page metadata.PageRequest) (metadata.Page[metadata.Mood], error) {
	stub.lastPage = page
	return metadata.Page[metadata.Mood]{Items: []metadata.Mood{}, Limit: page.Limit, Offset: page.Offset}, stub.err
}
