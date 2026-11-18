package artistimage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/coveringest"
	"github.com/google/uuid"
)

var artistNamespace = uuid.MustParse("bb6b7b23-c244-49ad-8747-09c37f47bdec")

type PermissionChecker interface {
	CanUploadArtistImage(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type Store interface {
	Replace(context.Context, Image) (*Image, error)
	Create(context.Context, Image) (bool, error)
	ByCatalogArtistID(context.Context, uuid.UUID) (Image, error)
}

type ObjectStore interface {
	PutIfAbsent(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type Service struct {
	permissions PermissionChecker
	store       Store
	objects     ObjectStore
}

func NewService(permissions PermissionChecker, store Store, objects ObjectStore) *Service {
	return &Service{permissions: permissions, store: store, objects: objects}
}

func (service *Service) Upload(
	ctx context.Context,
	actorID, creatorID uuid.UUID,
	contentType string,
	length int64,
	body io.Reader,
) (Image, error) {
	if actorID == uuid.Nil || creatorID == uuid.Nil || body == nil {
		return Image{}, ErrInvalid
	}
	allowed, err := service.permissions.CanUploadArtistImage(ctx, actorID, creatorID)
	if err != nil {
		if errors.Is(err, content.ErrForbidden) || errors.Is(err, content.ErrNotFound) {
			return Image{}, ErrForbidden
		}
		return Image{}, fmt.Errorf("authorize artist image: %w: %v", ErrUnavailable, err)
	}
	if !allowed {
		return Image{}, ErrForbidden
	}
	image, _, err := service.storeImage(ctx, actorID, &creatorID, CatalogArtistID(creatorID),
		"creator-content/"+creatorID.String(), contentType, length, body, true)
	return image, err
}

// Import stores provider artwork only while the creator does not already have
// an image. A concurrent manual upload always wins.
func (service *Service) Import(
	ctx context.Context,
	actorID, creatorID uuid.UUID,
	contentType string,
	length int64,
	body io.Reader,
) (Image, bool, error) {
	if actorID == uuid.Nil || creatorID == uuid.Nil || body == nil {
		return Image{}, false, ErrInvalid
	}
	allowed, err := service.permissions.CanUploadArtistImage(ctx, actorID, creatorID)
	if err != nil {
		if errors.Is(err, content.ErrForbidden) || errors.Is(err, content.ErrNotFound) {
			return Image{}, false, ErrForbidden
		}
		return Image{}, false, fmt.Errorf("authorize artist image: %w: %v", ErrUnavailable, err)
	}
	if !allowed {
		return Image{}, false, ErrForbidden
	}
	return service.storeImage(ctx, actorID, &creatorID, CatalogArtistID(creatorID),
		"provider-content/"+creatorID.String(), contentType, length, body, false)
}

func (service *Service) UploadCatalog(
	ctx context.Context,
	actorID uuid.UUID,
	roles []string,
	catalogArtistID uuid.UUID,
	contentType string,
	length int64,
	body io.Reader,
) (Image, error) {
	if actorID == uuid.Nil || catalogArtistID == uuid.Nil || body == nil {
		return Image{}, ErrInvalid
	}
	if !hasRole(roles, "admin") {
		return Image{}, ErrForbidden
	}
	image, _, err := service.storeImage(ctx, actorID, nil, catalogArtistID,
		"catalog-content", contentType, length, body, true)
	return image, err
}

func (service *Service) storeImage(
	ctx context.Context,
	actorID uuid.UUID,
	creatorID *uuid.UUID,
	catalogArtistID uuid.UUID,
	keyPrefix, contentType string,
	length int64,
	body io.Reader,
	replace bool,
) (Image, bool, error) {
	if length < 1 || length > MaximumBytes {
		return Image{}, false, ErrTooLarge
	}
	mimeType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || len(params) > 0 || !supportedMIME(mimeType) {
		return Image{}, false, ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(body, MaximumBytes+1))
	if err != nil {
		return Image{}, false, fmt.Errorf("read artist image: %w", err)
	}
	if int64(len(data)) > MaximumBytes {
		return Image{}, false, ErrTooLarge
	}
	if int64(len(data)) != length || coveringest.ValidateImage(data, mimeType) != nil {
		return Image{}, false, ErrInvalid
	}
	imageID := uuid.New()
	objectKey := fmt.Sprintf(
		"%s/artists/%s/images/%s/profile.%s",
		keyPrefix, catalogArtistID, imageID, extension(mimeType),
	)
	if err = service.objects.PutIfAbsent(ctx, objectKey, bytes.NewReader(data), length, mimeType); err != nil {
		return Image{}, false, fmt.Errorf("store artist image: %w: %v", ErrStorage, err)
	}
	image := Image{
		CreatorID: creatorID, CatalogArtistID: catalogArtistID,
		UpdatedByUserID: actorID, ObjectKey: objectKey,
		MIMEType: mimeType, SizeBytes: length,
	}
	var previous *Image
	created := true
	if replace {
		previous, err = service.store.Replace(ctx, image)
	} else {
		created, err = service.store.Create(ctx, image)
	}
	if err != nil {
		_ = service.objects.Delete(context.WithoutCancel(ctx), objectKey)
		return Image{}, false, fmt.Errorf("save artist image: %w", err)
	}
	if !created {
		_ = service.objects.Delete(context.WithoutCancel(ctx), objectKey)
		return Image{}, false, nil
	}
	if previous != nil && previous.ObjectKey != "" && previous.ObjectKey != objectKey {
		_ = service.objects.Delete(context.WithoutCancel(ctx), previous.ObjectKey)
	}
	return image, true, nil
}

func CatalogArtistID(creatorID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(artistNamespace, creatorID[:])
}

func hasRole(roles []string, expected string) bool {
	for _, role := range roles {
		if role == expected {
			return true
		}
	}
	return false
}

func (service *Service) Find(ctx context.Context, catalogArtistID uuid.UUID) (Image, error) {
	if catalogArtistID == uuid.Nil {
		return Image{}, ErrInvalid
	}
	return service.store.ByCatalogArtistID(ctx, catalogArtistID)
}

func supportedMIME(value string) bool {
	return value == "image/jpeg" || value == "image/png" || value == "image/webp"
}

func extension(value string) string {
	if value == "image/jpeg" {
		return "jpg"
	}
	return strings.TrimPrefix(value, "image/")
}

func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
