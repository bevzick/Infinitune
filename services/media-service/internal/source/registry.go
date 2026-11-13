package source

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type Registry struct {
	providers map[ProviderName]Provider
}

func (registry *Registry) Providers() []ProviderName {
	if registry == nil {
		return []ProviderName{}
	}
	providers := make([]ProviderName, 0, len(registry.providers))
	for name := range registry.providers {
		providers = append(providers, name)
	}
	sort.Slice(providers, func(left, right int) bool { return providers[left] < providers[right] })
	return providers
}

func (registry *Registry) Discover(
	ctx context.Context,
	providerName ProviderName,
	query DiscoveryQuery,
) (DiscoveryPage, error) {
	provider, err := registry.provider(providerName)
	if err != nil {
		return DiscoveryPage{}, err
	}
	query.Query = strings.TrimSpace(query.Query)
	if query.Query == "" || len(query.Query) > MaximumDiscoveryQuery {
		return DiscoveryPage{}, validationError("query", "must contain between 1 and 200 characters")
	}
	if query.Limit == 0 {
		query.Limit = DefaultDiscoveryLimit
	}
	if query.Limit < 1 || query.Limit > MaximumDiscoveryLimit || query.Offset < 0 {
		return DiscoveryPage{}, validationError("pagination", "is outside the supported range")
	}
	page, err := provider.Discover(ctx, query)
	if err != nil {
		return DiscoveryPage{}, fmt.Errorf("discover %s tracks: %w", providerName, err)
	}
	if page.Limit != query.Limit || page.Offset != query.Offset || page.Total < len(page.Tracks) {
		return DiscoveryPage{}, fmt.Errorf("%w: provider returned invalid discovery pagination", ErrInvalid)
	}
	for index, track := range page.Tracks {
		if err = ValidateTrack(track); err != nil {
			return DiscoveryPage{}, fmt.Errorf("validate %s discovery track %d: %w", providerName, index, err)
		}
	}
	return page, nil
}

func NewRegistry(providers ...Provider) (*Registry, error) {
	registry := &Registry{providers: make(map[ProviderName]Provider, len(providers))}
	for _, provider := range providers {
		if provider == nil || !provider.Name().Supported() {
			return nil, fmt.Errorf("%w: %v", ErrUnsupportedProvider, provider)
		}
		if _, exists := registry.providers[provider.Name()]; exists {
			return nil, fmt.Errorf("%w: %s", ErrProviderConflict, provider.Name())
		}
		registry.providers[provider.Name()] = provider
	}
	return registry, nil
}

func (registry *Registry) Resolve(
	ctx context.Context,
	providerName ProviderName,
	externalID string,
) (Track, error) {
	provider, err := registry.provider(providerName)
	if err != nil {
		return Track{}, err
	}
	if !validExternalID(externalID) {
		return Track{}, validationError("external_id", "must be a non-blank provider identifier")
	}
	track, err := provider.Resolve(ctx, externalID)
	if err != nil {
		return Track{}, fmt.Errorf("resolve %s track %q: %w", providerName, externalID, err)
	}
	if track.ExternalID != externalID {
		return Track{}, fmt.Errorf("%w: provider returned a different external_id", ErrInvalid)
	}
	if err = ValidateTrack(track); err != nil {
		return Track{}, fmt.Errorf("validate %s track %q: %w", providerName, externalID, err)
	}
	return track, nil
}

func (registry *Registry) Open(
	ctx context.Context,
	providerName ProviderName,
	externalID string,
) (AudioStream, error) {
	provider, err := registry.provider(providerName)
	if err != nil {
		return AudioStream{}, err
	}
	if !validExternalID(externalID) {
		return AudioStream{}, validationError("external_id", "must be a non-blank provider identifier")
	}
	stream, err := provider.Open(ctx, externalID)
	if err != nil {
		return AudioStream{}, fmt.Errorf("open %s track %q: %w", providerName, externalID, err)
	}
	if err = ValidateAudioStream(stream); err != nil {
		if stream.Body != nil {
			_ = stream.Body.Close()
		}
		return AudioStream{}, fmt.Errorf("validate %s track stream %q: %w",
			providerName, externalID, err)
	}
	return stream, nil
}

func (registry *Registry) provider(name ProviderName) (Provider, error) {
	if registry == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, name)
	}
	provider, exists := registry.providers[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, name)
	}
	return provider, nil
}
