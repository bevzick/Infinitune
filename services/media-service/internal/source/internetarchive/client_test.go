package internetarchive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/source"
)

func TestResolveMapsLicensedOriginalAudio(t *testing.T) {
	server := newArchiveServer(t, func(t *testing.T, request *http.Request) itemDocument {
		if request.URL.Query().Get("extended_err") != "1" {
			t.Errorf("extended_err = %q", request.URL.Query().Get("extended_err"))
		}
		return validItem()
	})
	defer server.Close()
	client := newTestClient(t, server)

	track, err := client.Resolve(context.Background(), "night-signals/audio/track one.mp3")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if track.ExternalID != "night-signals/audio/track one.mp3" ||
		track.Title != "Night Signal" || track.Artist != "Luna Veil" ||
		track.PageURL != "https://archive.org/details/night-signals" {
		t.Errorf("Resolve() = %+v", track)
	}
	if track.License.Name != "CC BY 4.0" ||
		track.License.URL != "https://creativecommons.org/licenses/by/4.0/" ||
		!track.License.AllowsStreaming || !track.License.AllowsRedistribution {
		t.Errorf("License = %+v", track.License)
	}
	if track.Metadata["item_identifier"] != "night-signals" ||
		track.Metadata["file_name"] != "audio/track one.mp3" ||
		track.Metadata["format"] != "VBR MP3" || track.Metadata["track_number"] != "1" {
		t.Errorf("Metadata = %#v", track.Metadata)
	}
	if err = source.ValidateTrack(track); err != nil {
		t.Errorf("ValidateTrack() error = %v", err)
	}
}

func TestOpenDownloadsSelectedFile(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/metadata/night-signals":
			writeItem(t, response, validItem())
		case "/download/night-signals/audio/track one.mp3":
			if request.Header.Get("Accept") != "audio/mpeg" {
				t.Errorf("Accept = %q", request.Header.Get("Accept"))
			}
			response.Header().Set("Content-Type", "application/octet-stream")
			_, _ = io.WriteString(response, "audio")
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server)

	stream, err := client.Open(context.Background(), "night-signals/audio/track one.mp3")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Body.Close()
	data, _ := io.ReadAll(stream.Body)
	if string(data) != "audio" || stream.MIMEType != "audio/mpeg" ||
		stream.SizeBytes != 5 || stream.Filename != "internet-archive-night-signals.mp3" {
		t.Errorf("stream = %+v, data = %q", stream, data)
	}
}

func TestResolveRejectsMissingDerivativeAndUnlicensedItems(t *testing.T) {
	tests := []struct {
		name string
		item any
		want error
	}{
		{name: "missing", item: []any{}, want: source.ErrNotFound},
		{name: "derivative", item: func() itemDocument {
			item := validItem()
			item.Files[0].Source = "derivative"
			return item
		}(), want: source.ErrInvalid},
		{name: "unsupported license", item: func() itemDocument {
			item := validItem()
			item.Metadata.LicenseURL = textValue{"https://example.com/custom"}
			return item
		}(), want: ErrUnsupportedLicense},
		{name: "not audio", item: func() itemDocument {
			item := validItem()
			item.Metadata.MediaType = "texts"
			return item
		}(), want: source.ErrInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(test.item)
			}))
			defer server.Close()
			client := newTestClient(t, server)
			_, err := client.Resolve(context.Background(), "night-signals/audio/track one.mp3")
			if !errors.Is(err, test.want) {
				t.Fatalf("Resolve() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestResolveRejectsInvalidExternalIdentifiers(t *testing.T) {
	client, err := New(Config{}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, externalID := range []string{
		"missing-file", "/track.mp3", "item/../track.mp3", "bad item/track.mp3",
	} {
		if _, err = client.Resolve(context.Background(), externalID); !errors.Is(err, source.ErrInvalid) {
			t.Errorf("Resolve(%q) error = %v", externalID, err)
		}
	}
}

func TestArchiveLicenseSupportsCC0(t *testing.T) {
	license, err := archiveLicense(
		"http://creativecommons.org/publicdomain/zero/1.0/?ref=archive#license", "Artist",
	)
	if err != nil {
		t.Fatalf("archiveLicense() error = %v", err)
	}
	if license.Name != "CC0 1.0" ||
		license.URL != "https://creativecommons.org/publicdomain/zero/1.0/" {
		t.Errorf("archiveLicense() = %+v", license)
	}
}

func newArchiveServer(
	t *testing.T,
	item func(*testing.T, *http.Request) itemDocument,
) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/metadata/night-signals" {
			http.NotFound(response, request)
			return
		}
		writeItem(t, response, item(t, request))
	}))
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	client, err := New(Config{
		MetadataURL: server.URL + "/metadata", DownloadURL: server.URL + "/download",
		Timeout: time.Second, AllowedHosts: []string{parsed.Hostname()},
	}, server.Client())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func writeItem(t *testing.T, response http.ResponseWriter, item itemDocument) {
	t.Helper()
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(item); err != nil {
		t.Errorf("Encode() error = %v", err)
	}
}

func validItem() itemDocument {
	return itemDocument{
		Metadata: itemMetadata{
			Identifier: "night-signals", MediaType: "audio",
			Title: textValue{"Night Signals Album"}, Creator: textValue{"Luna Veil"},
			LicenseURL: textValue{"http://creativecommons.org/licenses/by/4.0/"},
			Date:       textValue{"2026-11-10"},
		},
		Files: []fileRecord{{
			Name: "audio/track one.mp3", Source: "original", Format: "VBR MP3", Size: "5",
			Title: "Night Signal", Artist: "Luna Veil", Track: "1",
		}},
	}
}
