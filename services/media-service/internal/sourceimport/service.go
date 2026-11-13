package sourceimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/google/uuid"
)

var (
	ErrInvalid   = errors.New("invalid external media import")
	ErrForbidden = errors.New("external media import is forbidden")
	ErrConflict  = errors.New("release track already has a different external source")
	ErrLicense   = errors.New("external media license does not permit import")
	ErrStorage   = errors.New("external media storage failed")
)

const compensationTimeout = 5 * time.Second

type Authorizer interface {
	CanImport(context.Context, uuid.UUID, uuid.UUID) error
}

type Providers interface {
	Resolve(context.Context, source.ProviderName, string) (source.Track, error)
	Open(context.Context, source.ProviderName, string) (source.AudioStream, error)
}

type Store interface {
	ExternalByContentTrack(context.Context, uuid.UUID) (media.Asset, source.Reference, error)
	CreateExternal(context.Context, media.Asset, source.Reference) (media.Asset, bool, error)
}

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type Service struct {
	authorizer Authorizer
	providers  Providers
	store      Store
	objects    ObjectStore
}

func NewService(
	authorizer Authorizer,
	providers Providers,
	store Store,
	objects ObjectStore,
) *Service {
	return &Service{authorizer: authorizer, providers: providers, store: store, objects: objects}
}

func (service *Service) Import(
	ctx context.Context,
	actorID uuid.UUID,
	contentTrackID uuid.UUID,
	provider source.ProviderName,
	externalID string,
) (media.Asset, bool, error) {
	if actorID == uuid.Nil || contentTrackID == uuid.Nil || !provider.Supported() ||
		externalID == "" || externalID != strings.TrimSpace(externalID) ||
		len(externalID) > source.MaximumExternalIDLength || strings.ContainsAny(externalID, "\r\n\t") {
		return media.Asset{}, false, ErrInvalid
	}
	if err := service.authorizer.CanImport(ctx, actorID, contentTrackID); err != nil {
		if errors.Is(err, content.ErrForbidden) {
			return media.Asset{}, false, ErrForbidden
		}
		return media.Asset{}, false, fmt.Errorf("authorize external import: %w", err)
	}
	if existing, reference, err := service.store.ExternalByContentTrack(ctx, contentTrackID); err == nil {
		if reference.Provider != provider || reference.ExternalID != externalID {
			return media.Asset{}, false, ErrConflict
		}
		return existing, false, nil
	} else if !errors.Is(err, media.ErrNotFound) {
		return media.Asset{}, false, fmt.Errorf("find existing external import: %w", err)
	}
	track, err := service.providers.Resolve(ctx, provider, externalID)
	if err != nil {
		return media.Asset{}, false, err
	}
	if !track.License.AllowsStreaming || !track.License.AllowsRedistribution {
		return media.Asset{}, false, ErrLicense
	}
	stream, err := service.providers.Open(ctx, provider, externalID)
	if err != nil {
		return media.Asset{}, false, err
	}
	defer stream.Body.Close()
	assetID := uuid.New()
	objectKey := fmt.Sprintf("external-imports/%s/%s", assetID, stream.Filename)
	if err = service.objects.Put(
		ctx, objectKey, stream.Body, stream.SizeBytes, stream.MIMEType,
	); err != nil {
		return media.Asset{}, false, fmt.Errorf("store external audio: %w: %v", ErrStorage, err)
	}
	asset := media.Asset{
		ID: assetID, ContentTrackID: contentTrackID,
		OriginalObjectKey: objectKey, OriginalFilename: stream.Filename,
		Status: media.StatusUploaded,
	}
	metadata := maps.Clone(track.Metadata)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["title"] = track.Title
	metadata["artist"] = track.Artist
	reference := source.Reference{
		Kind: source.KindExternalProvider, Provider: provider,
		ExternalID: externalID, SourceURL: track.PageURL,
		License: track.License, Metadata: metadata,
	}
	stored, created, err := service.store.CreateExternal(ctx, asset, reference)
	if err != nil || !created {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), compensationTimeout)
		defer cancel()
		deleteErr := service.objects.Delete(cleanupCtx, objectKey)
		if err != nil {
			if errors.Is(err, source.ErrConflict) {
				err = ErrConflict
			}
			return media.Asset{}, false, errors.Join(err, deleteErr)
		}
		return stored, false, deleteErr
	}
	return stored, true, nil
}
