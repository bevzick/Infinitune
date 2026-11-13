package jamendo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/source"
)

func TestResolveMapsJamendoTrack(t *testing.T) {
	server := newJamendoServer(t, func(t *testing.T, request *http.Request) trackResult {
		assertTrackRequest(t, request, "42")
		return validTrack("42", "https://storage.jamendo.com/download/track.mp3")
	})
	defer server.Close()
	client := newTestClient(t, server)

	track, err := client.Resolve(context.Background(), "42")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if track.ExternalID != "42" || track.Title != "Night Signal" || track.Artist != "Luna Veil" {
		t.Errorf("Resolve() track = %+v", track)
	}
	if track.PageURL != "https://www.jamendo.com/track/42" {
		t.Errorf("PageURL = %q", track.PageURL)
	}
	if track.License.Name != "CC BY 4.0" ||
		track.License.URL != "https://creativecommons.org/licenses/by/4.0/" ||
		track.License.Attribution != "Luna Veil" || !track.License.AllowsStreaming ||
		!track.License.AllowsRedistribution {
		t.Errorf("License = %+v", track.License)
	}
	if track.Metadata["artist_id"] != "artist-7" || track.Metadata["album_id"] != "album-9" ||
		track.Metadata["album_name"] != "After Dark" ||
		track.Metadata["duration_sec"] != "228" ||
		track.Metadata["release_date"] != "2026-11-10" {
		t.Errorf("Metadata = %#v", track.Metadata)
	}
	if err = source.ValidateTrack(track); err != nil {
		t.Errorf("ValidateTrack() error = %v", err)
	}
}

func TestOpenReturnsAllowedAudio(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v3.0/tracks/":
			assertTrackRequest(t, request, "42")
			writeTrackResponse(t, response, validTrack("42", server.URL+"/audio/42.mp3"))
		case "/audio/42.mp3":
			if request.Header.Get("Accept") != "audio/mpeg" {
				t.Errorf("download Accept = %q", request.Header.Get("Accept"))
			}
			response.Header().Set("Content-Type", "audio/mpeg")
			_, _ = io.WriteString(response, "mock-mp3")
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server)

	stream, err := client.Open(context.Background(), "42")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Body.Close()
	contents, err := io.ReadAll(stream.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(contents) != "mock-mp3" || stream.Filename != "jamendo-42.mp3" ||
		stream.MIMEType != "audio/mpeg" || stream.SizeBytes != int64(len(contents)) {
		t.Errorf("Open() stream = %+v, contents = %q", stream, contents)
	}
}

func TestOpenRejectsForbiddenAndUnsafeDownloads(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*trackResult)
		want   error
	}{
		{
			name: "provider forbids download",
			modify: func(track *trackResult) {
				track.DownloadAllowed = false
				track.DownloadURL = ""
			},
			want: ErrDownloadForbidden,
		},
		{
			name: "unapproved download host",
			modify: func(track *trackResult) {
				track.DownloadURL = "https://untrusted.example/audio.mp3"
			},
			want: ErrUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newJamendoServer(t, func(_ *testing.T, _ *http.Request) trackResult {
				track := validTrack("42", "https://storage.jamendo.com/audio.mp3")
				test.modify(&track)
				return track
			})
			defer server.Close()
			client := newTestClient(t, server)
			if _, err := client.Open(context.Background(), "42"); !errors.Is(err, test.want) {
				t.Fatalf("Open() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestResolveHandlesMissingInvalidAndUnsupportedTracks(t *testing.T) {
	t.Run("invalid identifier", func(t *testing.T) {
		client, err := New(Config{ClientID: "client-id"}, nil)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if _, err = client.Resolve(context.Background(), "track/42"); !errors.Is(err, source.ErrInvalid) {
			t.Fatalf("Resolve() error = %v, want %v", err, source.ErrInvalid)
		}
	})

	t.Run("missing track", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(response).Encode(tracksResponse{
				Headers: responseHeaders{Status: "success"}, Results: []trackResult{},
			})
		}))
		defer server.Close()
		client := newTestClient(t, server)
		if _, err := client.Resolve(context.Background(), "42"); !errors.Is(err, source.ErrNotFound) {
			t.Fatalf("Resolve() error = %v, want %v", err, source.ErrNotFound)
		}
	})

	t.Run("unsupported license", func(t *testing.T) {
		server := newJamendoServer(t, func(_ *testing.T, _ *http.Request) trackResult {
			track := validTrack("42", "https://storage.jamendo.com/audio.mp3")
			track.LicenseURL = "https://example.com/custom-license"
			return track
		})
		defer server.Close()
		client := newTestClient(t, server)
		if _, err := client.Resolve(context.Background(), "42"); !errors.Is(err, ErrUnsupportedLicense) {
			t.Fatalf("Resolve() error = %v, want %v", err, ErrUnsupportedLicense)
		}
	})
}

func newJamendoServer(
	t *testing.T,
	result func(*testing.T, *http.Request) trackResult,
) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v3.0/tracks/" {
			http.NotFound(response, request)
			return
		}
		writeTrackResponse(t, response, result(t, request))
	}))
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	client, err := New(Config{
		ClientID: "client-id", BaseURL: server.URL + "/v3.0", Timeout: time.Second,
		DownloadHosts: []string{parsed.Hostname()},
	}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func assertTrackRequest(t *testing.T, request *http.Request, trackID string) {
	t.Helper()
	query := request.URL.Query()
	want := map[string]string{
		"client_id": "client-id", "format": "json", "limit": "1", "id": trackID,
		"include": "licenses", "audioformat": "mp32", "audiodlformat": "mp32",
	}
	for key, expected := range want {
		if actual := query.Get(key); actual != expected {
			t.Errorf("query %s = %q, want %q", key, actual, expected)
		}
	}
}

func writeTrackResponse(t *testing.T, response http.ResponseWriter, result trackResult) {
	t.Helper()
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(tracksResponse{
		Headers: responseHeaders{Status: "success", Code: 0}, Results: []trackResult{result},
	}); err != nil {
		t.Errorf("Encode() error = %v", err)
	}
}

func validTrack(id, downloadURL string) trackResult {
	return trackResult{
		ID: id, Name: "Night Signal", Duration: 228,
		ArtistID: "artist-7", ArtistName: "Luna Veil",
		AlbumID: "album-9", AlbumName: "After Dark", ReleaseDate: "2026-11-10",
		LicenseURL:  "http://creativecommons.org/licenses/by/4.0/",
		ShareURL:    "https://www.jamendo.com/track/" + id,
		DownloadURL: downloadURL, DownloadAllowed: true,
	}
}

func TestLicenseFromURL(t *testing.T) {
	license, err := licenseFromURL(
		"https://www.creativecommons.org/licenses/by-nc-sa/3.0/?ignored=true#fragment",
		"Artist", false,
	)
	if err != nil {
		t.Fatalf("licenseFromURL() error = %v", err)
	}
	if license.Name != "CC BY-NC-SA 3.0" ||
		license.URL != "https://www.creativecommons.org/licenses/by-nc-sa/3.0/" ||
		license.AllowsRedistribution {
		t.Errorf("licenseFromURL() = %+v", license)
	}

	for _, rawURL := range []string{
		"https://example.com/licenses/by/4.0/",
		"https://creativecommons.org/licenses/custom/1.0/",
		strings.Repeat("x", 10),
	} {
		if _, err := licenseFromURL(rawURL, "Artist", true); !errors.Is(err, ErrUnsupportedLicense) {
			t.Errorf("licenseFromURL(%q) error = %v", rawURL, err)
		}
	}
}
