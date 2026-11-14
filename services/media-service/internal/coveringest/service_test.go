package coveringest

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os/exec"
	"testing"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/google/uuid"
)

type sessionFake struct {
	session     content.CoverSession
	transitions []content.Status
	completeErr error
}

func (f *sessionFake) Session(_ context.Context, actorID, uploadID uuid.UUID) (content.CoverSession, error) {
	if f.session.RequestedByUserID != actorID || f.session.ID != uploadID {
		return content.CoverSession{}, content.ErrForbidden
	}
	return f.session, nil
}
func (f *sessionFake) Transition(_ context.Context, id uuid.UUID, next content.Status) (content.CoverSession, error) {
	if id != f.session.ID {
		return content.CoverSession{}, content.ErrNotFound
	}
	if (next == content.StatusUploading && f.session.Status != content.StatusPending) ||
		(next == content.StatusFailed && f.session.Status != content.StatusUploading) {
		return content.CoverSession{}, content.ErrConflict
	}
	f.session.Status = next
	f.transitions = append(f.transitions, next)
	return f.session, nil
}
func (f *sessionFake) Complete(_ context.Context, id uuid.UUID) (content.CoverSession, error) {
	if f.completeErr != nil {
		return content.CoverSession{}, f.completeErr
	}
	if f.session.Status != content.StatusUploading {
		return content.CoverSession{}, content.ErrInvalidTransition
	}
	f.session.Status = content.StatusCompleted
	return f.session, nil
}

type objectFake struct {
	data                []byte
	key, typ            string
	puts, deletes       int
	putErr, errorDelete error
}

func (o *objectFake) PutIfAbsent(_ context.Context, key string, body io.Reader, _ int64, typ string) error {
	o.puts++
	o.key = key
	o.typ = typ
	if o.putErr != nil {
		return o.putErr
	}
	o.data, _ = io.ReadAll(body)
	return nil
}
func (o *objectFake) Delete(_ context.Context, _ string) error { o.deletes++; return o.errorDelete }
func coverPNG() []byte {
	imageData := image.NewRGBA(image.Rect(0, 0, 4, 4))
	imageData.Set(0, 0, color.RGBA{R: 250, A: 255})
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, imageData)
	return buffer.Bytes()
}
func newSession(size int) (*sessionFake, *objectFake, uuid.UUID) {
	actor, upload, release, creator := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	session := content.CoverSession{
		ID: upload, ReleaseID: release, RequestedByUserID: actor,
		ObjectKey: "creator-content/" + creator.String() + "/" + release.String() + "/covers/" + upload.String() + "/cover.png",
		MIMEType:  "image/png", SizeBytes: int64(size), Status: content.StatusPending,
	}
	return &sessionFake{session: session}, &objectFake{}, actor
}
func TestCoverIngestCompletesAndDoesNotOverwrite(t *testing.T) {
	image := coverPNG()
	sessions, objects, actor := newSession(len(image))
	service := NewService(sessions, objects)
	result, created, err := service.Ingest(context.Background(), actor, sessions.session.ID, "image/png", int64(len(image)), bytes.NewReader(image))
	if err != nil || !created || result.ReleaseID != sessions.session.ReleaseID || result.Status != content.StatusCompleted {
		t.Fatalf("result=%+v created=%v err=%v", result, created, err)
	}
	if objects.puts != 1 || objects.key != sessions.session.ObjectKey || objects.typ != "image/png" || !bytes.Equal(objects.data, image) {
		t.Fatal("image not stored under session key")
	}
	second, created, err := service.Ingest(context.Background(), actor, sessions.session.ID, "image/png", int64(len(image)), bytes.NewReader(image))
	if err != nil || created || second.Status != content.StatusCompleted || objects.puts != 1 {
		t.Fatalf("idempotency: %+v %v %v", second, created, err)
	}
}
func TestCoverIngestRejectsInvalidMetadataAndImage(t *testing.T) {
	image := coverPNG()
	tests := []struct {
		name, mime string
		bytes      []byte
		size       int64
	}{
		{"different MIME", "image/jpeg", image, int64(len(image))},
		{"wrong size", "image/png", image, int64(len(image) + 1)},
		{"faked png", "image/png", []byte("not an image"), int64(len("not an image"))},
		{"truncated png", "image/png", image[:len(image)/2], int64(len(image) - 8)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sessions, objects, actor := newSession(len(tc.bytes))
			_, _, err := NewService(sessions, objects).Ingest(context.Background(), actor, sessions.session.ID, tc.mime, tc.size, bytes.NewReader(tc.bytes))
			if !errors.Is(err, ErrInvalid) || objects.puts != 0 || len(sessions.transitions) != 0 {
				t.Fatalf("err=%v puts=%d transitions=%v", err, objects.puts, sessions.transitions)
			}
		})
	}
	if validDimensions(8193, 10) || validDimensions(8192, 8192) || validDimensions(0, 1) || !validDimensions(2000, 2000) {
		t.Fatal("invalid limits")
	}
}
func TestCoverIngestForbidsForeignActorAndUntrustedKey(t *testing.T) {
	image := coverPNG()
	sessions, objects, _ := newSession(len(image))
	_, _, err := NewService(sessions, objects).Ingest(context.Background(), uuid.New(), sessions.session.ID, "image/png", int64(len(image)), bytes.NewReader(image))
	if !errors.Is(err, content.ErrForbidden) || objects.puts != 0 {
		t.Fatalf("foreign actor: %v", err)
	}
	actor := sessions.session.RequestedByUserID
	sessions.session.ObjectKey = "public/cover.png"
	_, _, err = NewService(sessions, objects).Ingest(context.Background(), actor, sessions.session.ID, "image/png", int64(len(image)), bytes.NewReader(image))
	if !errors.Is(err, ErrInvalidState) || objects.puts != 0 {
		t.Fatalf("untrusted key: %v", err)
	}
}
func TestCoverIngestFailsCleanlyOnPutError(t *testing.T) {
	image := coverPNG()
	sessions, objects, actor := newSession(len(image))
	objects.putErr = errors.New("minio down")
	_, created, err := NewService(sessions, objects).Ingest(context.Background(), actor, sessions.session.ID, "image/png", int64(len(image)), bytes.NewReader(image))
	if !errors.Is(err, ErrStorage) || created || sessions.session.Status != content.StatusFailed {
		t.Fatalf("err=%v created=%v status=%v", err, created, sessions.session.Status)
	}
	_, _, err = NewService(sessions, objects).Ingest(context.Background(), actor, sessions.session.ID, "image/png", int64(len(image)), bytes.NewReader(image))
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("retry should use new session: %v", err)
	}
}
func TestCoverIngestCleansUpOnRejectedCompletion(t *testing.T) {
	image := coverPNG()
	sessions, objects, actor := newSession(len(image))
	sessions.completeErr = content.ErrInvalidTransition
	_, _, err := NewService(sessions, objects).Ingest(context.Background(), actor, sessions.session.ID, "image/png", int64(len(image)), bytes.NewReader(image))
	if !errors.Is(err, content.ErrInvalidTransition) || objects.deletes != 1 || sessions.session.Status != content.StatusFailed {
		t.Fatalf("err=%v deletes=%d status=%v", err, objects.deletes, sessions.session.Status)
	}
}
func TestWebPDecoderAndDimensions(t *testing.T) {
	// Tiny valid lossy WebP produced by ffmpeg; ffmpeg/ffprobe ship in media image.
	const encoded = "UklGRkwAAABXRUJQVlA4IEAAAABQAwCdASogACAAPpFCnEolo6KhqAgAsBIJZQDGqoAAQFEcAAD+7tPf/uOwN14t//ucD/ucD/ucD+NspCdnCgAA"
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if err = validateImage(data, "image/webp"); err != nil {
		t.Fatal(err)
	}
	if err = validateImage(data[:len(data)-8], "image/webp"); err == nil {
		t.Fatal("truncated WebP accepted")
	}
}
