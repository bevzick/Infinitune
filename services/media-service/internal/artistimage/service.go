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
	if length < 1 || length > MaximumBytes {
		return Image{}, ErrTooLarge
	}
	mimeType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || len(params) > 0 || !supportedMIME(mimeType) {
		return Image{}, ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(body, MaximumBytes+1))
	if err != nil {
		return Image{}, fmt.Errorf("read artist image: %w", err)
	}
	if int64(len(data)) > MaximumBytes {
		return Image{}, ErrTooLarge
	}
	if int64(len(data)) != length || coveringest.ValidateImage(data, mimeType) != nil {
		return Image{}, ErrInvalid
	}
	catalogArtistID := uuid.NewSHA1(artistNamespace, creatorID[:])
	imageID := uuid.New()
	objectKey := fmt.Sprintf(
		"creator-content/%s/artists/%s/images/%s/profile.%s",
		creatorID, catalogArtistID, imageID, extension(mimeType),
	)
	if err = service.objects.PutIfAbsent(ctx, objectKey, bytes.NewReader(data), length, mimeType); err != nil {
		return Image{}, fmt.Errorf("store artist image: %w: %v", ErrStorage, err)
	}
	image := Image{
		CreatorID: creatorID, CatalogArtistID: catalogArtistID,
		UpdatedByUserID: actorID, ObjectKey: objectKey,
		MIMEType: mimeType, SizeBytes: length,
	}
	previous, err := service.store.Replace(ctx, image)
	if err != nil {
		_ = service.objects.Delete(context.WithoutCancel(ctx), objectKey)
		return Image{}, fmt.Errorf("save artist image: %w", err)
	}
	if previous != nil && previous.ObjectKey != "" && previous.ObjectKey != objectKey {
		_ = service.objects.Delete(context.WithoutCancel(ctx), previous.ObjectKey)
	}
	return image, nil
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
