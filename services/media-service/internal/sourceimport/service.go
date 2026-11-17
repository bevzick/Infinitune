package sourceimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/artistimage"
	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/google/uuid"
)

var (
	ErrInvalid       = errors.New("invalid external media import")
	ErrForbidden     = errors.New("external media import is forbidden")
	ErrConflict      = errors.New("release track already has a different external source")
	ErrLicense       = errors.New("external media license does not permit import")
	ErrStorage       = errors.New("external media storage failed")
	ErrArtwork       = errors.New("external artwork import failed")
	ErrArtistArtwork = errors.New("external artist artwork import failed")
)

const compensationTimeout = 5 * time.Second

type Authorizer interface {
	CanImport(context.Context, uuid.UUID, uuid.UUID) (content.ImportTarget, error)
}

type Providers interface {
	Resolve(context.Context, source.ProviderName, string) (source.Track, error)
	Open(context.Context, source.ProviderName, string) (source.AudioStream, error)
	OpenArtwork(context.Context, source.ProviderName, string) (source.ArtworkStream, error)
	OpenArtistArtwork(context.Context, source.ProviderName, string) (source.ArtworkStream, error)
}

type CoverSessions interface {
	CreateImported(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, int64) (content.CoverSession, error)
	Transition(context.Context, uuid.UUID, content.Status) (content.CoverSession, error)
	CompleteIfMissing(context.Context, uuid.UUID) (content.CoverSession, error)
}

type Store interface {
	ExternalByContentTrack(context.Context, uuid.UUID) (media.Asset, source.Reference, error)
	CreateExternal(context.Context, media.Asset, source.Reference) (media.Asset, bool, error)
}

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type ArtistImages interface {
	Find(context.Context, uuid.UUID) (artistimage.Image, error)
	Import(context.Context, uuid.UUID, uuid.UUID, string, int64, io.Reader) (artistimage.Image, bool, error)
}

type Service struct {
	authorizer   Authorizer
	providers    Providers
	store        Store
	objects      ObjectStore
	covers       CoverSessions
	artistImages ArtistImages
}

func NewService(
	authorizer Authorizer,
	providers Providers,
	store Store,
	objects ObjectStore,
	covers CoverSessions,
	artistImages ...ArtistImages,
) *Service {
	service := &Service{
		authorizer: authorizer, providers: providers, store: store,
		objects: objects, covers: covers,
	}
	if len(artistImages) > 0 {
		service.artistImages = artistImages[0]
	}
	return service
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
	target, err := service.authorizer.CanImport(ctx, actorID, contentTrackID)
	if err != nil {
		if errors.Is(err, content.ErrForbidden) {
			return media.Asset{}, false, ErrForbidden
		}
		return media.Asset{}, false, fmt.Errorf("authorize external import: %w", err)
	}
	track, err := service.providers.Resolve(ctx, provider, externalID)
	if err != nil {
		return media.Asset{}, false, err
	}
	if !track.License.AllowsStreaming || !track.License.AllowsRedistribution {
		return media.Asset{}, false, ErrLicense
	}
	if existing, reference, err := service.store.ExternalByContentTrack(ctx, contentTrackID); err == nil {
		if reference.Provider != provider || reference.ExternalID != externalID {
			return media.Asset{}, false, ErrConflict
		}
		if err = service.importArtwork(ctx, actorID, target, provider, externalID, track); err != nil {
			return media.Asset{}, false, err
		}
		if err = service.importArtistArtwork(ctx, actorID, target, provider, externalID); err != nil {
			return media.Asset{}, false, err
		}
		return existing, false, nil
	} else if !errors.Is(err, media.ErrNotFound) {
		return media.Asset{}, false, fmt.Errorf("find existing external import: %w", err)
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
	if err = service.importArtwork(ctx, actorID, target, provider, externalID, track); err != nil {
		return stored, true, err
	}
	if err = service.importArtistArtwork(ctx, actorID, target, provider, externalID); err != nil {
		return stored, true, err
	}
	return stored, true, nil
}

func (service *Service) importArtistArtwork(
	ctx context.Context,
	actorID uuid.UUID,
	target content.ImportTarget,
	provider source.ProviderName,
	externalID string,
) error {
	if service.artistImages == nil || target.CreatorID == uuid.Nil {
		return nil
	}
	catalogArtistID := artistimage.CatalogArtistID(target.CreatorID)
	if _, err := service.artistImages.Find(ctx, catalogArtistID); err == nil {
		return nil
	} else if !errors.Is(err, artistimage.ErrNotFound) {
		return fmt.Errorf("%w: check existing artist image: %v", ErrArtistArtwork, err)
	}
	artwork, err := service.providers.OpenArtistArtwork(ctx, provider, externalID)
	if errors.Is(err, source.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: open provider artist artwork: %v", ErrArtistArtwork, err)
	}
	defer artwork.Body.Close()
	_, _, err = service.artistImages.Import(
		ctx, actorID, target.CreatorID, artwork.MIMEType,
		artwork.SizeBytes, artwork.Body,
	)
	if err != nil {
		return fmt.Errorf("%w: store provider artist artwork: %v", ErrArtistArtwork, err)
	}
	return nil
}

func (service *Service) importArtwork(
	ctx context.Context,
	actorID uuid.UUID,
	target content.ImportTarget,
	provider source.ProviderName,
	externalID string,
	track source.Track,
) error {
	if target.HasCover || track.ArtworkURL == "" || service.covers == nil {
		return nil
	}
	artwork, err := service.providers.OpenArtwork(ctx, provider, externalID)
	if errors.Is(err, source.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: open provider artwork: %v", ErrArtwork, err)
	}
	defer artwork.Body.Close()
	idempotencyKey := uuid.NewSHA1(uuid.NameSpaceURL, []byte(
		"infinitune:provider-artwork:"+target.ReleaseID.String()+":"+string(provider)+":"+externalID,
	))
	session, err := service.covers.CreateImported(
		ctx, actorID, target.ReleaseID, idempotencyKey,
		artwork.Filename, artwork.MIMEType, artwork.SizeBytes,
	)
	if err != nil {
		return fmt.Errorf("%w: create cover session: %v", ErrArtwork, err)
	}
	if session.Status == content.StatusCompleted {
		return nil
	}
	if session.Status == content.StatusFailed {
		session, err = service.covers.Transition(ctx, session.ID, content.StatusPending)
		if err != nil {
			return fmt.Errorf("%w: reset cover session: %v", ErrArtwork, err)
		}
	}
	if session.Status == content.StatusPending {
		session, err = service.covers.Transition(ctx, session.ID, content.StatusUploading)
		if err != nil {
			return fmt.Errorf("%w: start cover session: %v", ErrArtwork, err)
		}
	}
	if session.Status != content.StatusUploading {
		return fmt.Errorf("%w: unsupported cover session status %q", ErrArtwork, session.Status)
	}
	if err = service.objects.Put(ctx, session.ObjectKey, artwork.Body,
		artwork.SizeBytes, artwork.MIMEType); err != nil {
		_, _ = service.covers.Transition(context.WithoutCancel(ctx), session.ID, content.StatusFailed)
		return fmt.Errorf("%w: store provider artwork: %v", ErrArtwork, err)
	}
	if _, err = service.covers.CompleteIfMissing(ctx, session.ID); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), compensationTimeout)
		defer cancel()
		deleteErr := service.objects.Delete(cleanupCtx, session.ObjectKey)
		if errors.Is(err, content.ErrConflict) {
			return deleteErr
		}
		return errors.Join(fmt.Errorf("%w: attach provider artwork: %v", ErrArtwork, err), deleteErr)
	}
	return nil
}
