package artistimage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/google/uuid"
)

type permissionStub struct{ allowed bool }

func (stub permissionStub) CanUploadArtistImage(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return stub.allowed, nil
}

type storeStub struct {
	saved    Image
	previous *Image
}

func (stub *storeStub) Replace(_ context.Context, image Image) (*Image, error) {
	stub.saved = image
	return stub.previous, nil
}
func (stub *storeStub) ByCatalogArtistID(context.Context, uuid.UUID) (Image, error) {
	return stub.saved, nil
}

type objectStub struct {
	key, deleted, mime string
	data               []byte
}

func (stub *objectStub) PutIfAbsent(_ context.Context, key string, reader io.Reader, _ int64, mime string) error {
	stub.key, stub.mime = key, mime
	stub.data, _ = io.ReadAll(reader)
	return nil
}
func (stub *objectStub) Delete(_ context.Context, key string) error {
	stub.deleted = key
	return nil
}

func TestUploadStoresAndReplacesArtistImage(t *testing.T) {
	creatorID, actorID := uuid.New(), uuid.New()
	data := testPNG(t)
	store := &storeStub{previous: &Image{ObjectKey: "old/image.png"}}
	objects := &objectStub{}
	result, err := NewService(permissionStub{allowed: true}, store, objects).Upload(
		context.Background(), actorID, creatorID, "image/png", int64(len(data)), bytes.NewReader(data),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantArtistID := uuid.NewSHA1(artistNamespace, creatorID[:])
	if result.CatalogArtistID != wantArtistID || store.saved.UpdatedByUserID != actorID ||
		objects.mime != "image/png" || !bytes.Equal(objects.data, data) || objects.deleted != "old/image.png" {
		t.Fatalf("Upload() = %+v; saved = %+v; object = %+v", result, store.saved, objects)
	}
}

func TestUploadRejectsForbiddenAndMismatchedImage(t *testing.T) {
	data := testPNG(t)
	_, err := NewService(permissionStub{}, &storeStub{}, &objectStub{}).Upload(
		context.Background(), uuid.New(), uuid.New(), "image/png", int64(len(data)), bytes.NewReader(data),
	)
	if err != ErrForbidden {
		t.Fatalf("forbidden Upload() error = %v", err)
	}
	_, err = NewService(permissionStub{allowed: true}, &storeStub{}, &objectStub{}).Upload(
		context.Background(), uuid.New(), uuid.New(), "image/jpeg", int64(len(data)), bytes.NewReader(data),
	)
	if err != ErrInvalid {
		t.Fatalf("mismatched Upload() error = %v", err)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imageData.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&output, imageData); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
