package source

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRegistryResolvesAndOpensRegisteredProvider(t *testing.T) {
	provider := &providerStub{name: ProviderJamendo}
	registry, err := NewRegistry(provider)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	track, err := registry.Resolve(context.Background(), ProviderJamendo, "42")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if track.ExternalID != "42" || provider.resolvedID != "42" {
		t.Fatalf("resolved track = %+v, provider ID = %q", track, provider.resolvedID)
	}
	stream, err := registry.Open(context.Background(), ProviderJamendo, "42")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Body.Close()
	if provider.openedID != "42" || stream.MIMEType != "audio/mpeg" {
		t.Fatalf("stream = %+v, provider ID = %q", stream, provider.openedID)
	}
}

func TestRegistryListsAndDiscoversRegisteredProviders(t *testing.T) {
	provider := &providerStub{name: ProviderJamendo}
	registry, err := NewRegistry(provider)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if got := registry.Providers(); len(got) != 1 || got[0] != ProviderJamendo {
		t.Fatalf("Providers() = %v", got)
	}
	page, err := registry.Discover(context.Background(), ProviderJamendo, DiscoveryQuery{Query: "focus"})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if provider.discovery.Query != "focus" || provider.discovery.Limit != DefaultDiscoveryLimit ||
		len(page.Tracks) != 1 {
		t.Fatalf("query = %+v, page = %+v", provider.discovery, page)
	}
}

func TestRegistryRejectsDuplicateAndUnregisteredProviders(t *testing.T) {
	_, err := NewRegistry(
		&providerStub{name: ProviderJamendo},
		&providerStub{name: ProviderJamendo},
	)
	if !errors.Is(err, ErrProviderConflict) {
		t.Fatalf("NewRegistry(duplicate) error = %v, want %v", err, ErrProviderConflict)
	}
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	_, err = registry.Resolve(context.Background(), ProviderInternetArchive, "track")
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Fatalf("Resolve(unregistered) error = %v, want %v", err, ErrUnsupportedProvider)
	}
}

func TestRegistryClosesInvalidProviderStream(t *testing.T) {
	body := &trackingReadCloser{Reader: strings.NewReader("not audio")}
	registry, err := NewRegistry(&providerStub{
		name: ProviderJamendo,
		stream: AudioStream{
			Body: body, Filename: "track.html", MIMEType: "text/html", SizeBytes: 9,
		},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	_, err = registry.Open(context.Background(), ProviderJamendo, "42")
	if !errors.Is(err, ErrInvalidStream) {
		t.Fatalf("Open() error = %v, want %v", err, ErrInvalidStream)
	}
	if !body.closed {
		t.Fatal("invalid provider stream body was not closed")
	}
}

type providerStub struct {
	name       ProviderName
	stream     AudioStream
	resolvedID string
	openedID   string
	discovery  DiscoveryQuery
}

func (stub *providerStub) Name() ProviderName { return stub.name }

func (stub *providerStub) Discover(_ context.Context, query DiscoveryQuery) (DiscoveryPage, error) {
	stub.discovery = query
	return DiscoveryPage{Tracks: []Track{{
		ExternalID: "42", Title: "Test Track", Artist: "Test Artist",
		PageURL: "https://example.com/tracks/42",
		License: License{Name: "CC BY", URL: "https://example.com/licenses/cc-by"},
	}}, Limit: query.Limit, Offset: query.Offset, Total: 1}, nil
}

func (stub *providerStub) Resolve(_ context.Context, externalID string) (Track, error) {
	stub.resolvedID = externalID
	return Track{
		ExternalID: externalID, Title: "Test Track", Artist: "Test Artist",
		PageURL: "https://example.com/tracks/" + externalID,
		License: License{Name: "CC BY", URL: "https://example.com/licenses/cc-by"},
	}, nil
}

func (stub *providerStub) Open(_ context.Context, externalID string) (AudioStream, error) {
	stub.openedID = externalID
	if stub.stream.Body != nil {
		return stub.stream, nil
	}
	return AudioStream{
		Body:     io.NopCloser(strings.NewReader("audio")),
		Filename: "track.mp3", MIMEType: "audio/mpeg", SizeBytes: 5,
	}, nil
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (body *trackingReadCloser) Close() error {
	body.closed = true
	return nil
}
