package source

import (
	"context"
	"fmt"
)

type Registry struct {
	providers map[ProviderName]Provider
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
