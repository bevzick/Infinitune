package ingest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/google/uuid"
)

func TestServiceIngestsOriginalAndCompletesSession(t *testing.T) {
	actorID, uploadID, trackID := uuid.New(), uuid.New(), uuid.New()
	contentUploads := &contentStub{session: uploadSession(actorID, uploadID, trackID)}
	assets := &assetStoreStub{findErr: media.ErrNotFound}
	objects := &objectStoreStub{}
	asset, created, err := NewService(contentUploads, assets, objects).Ingest(
		context.Background(), actorID, uploadID, "audio/flac", 5,
		bytes.NewBufferString("audio"),
	)
	if err != nil || !created {
		t.Fatalf("Ingest() = %+v, created %v, %v", asset, created, err)
	}
	if asset.ContentTrackID != trackID || asset.SourceUploadID != uploadID ||
		asset.Status != media.StatusUploaded || string(objects.data) != "audio" {
		t.Fatalf("asset = %+v, stored data = %q", asset, objects.data)
	}
	if len(contentUploads.transitions) != 2 ||
		contentUploads.transitions[0] != content.StatusUploading ||
		contentUploads.transitions[1] != content.StatusCompleted {
		t.Fatalf("transitions = %v", contentUploads.transitions)
	}
	if objects.key != contentUploads.session.ObjectKey || objects.contentType != "audio/flac" {
		t.Fatalf("stored object = key %q, type %q", objects.key, objects.contentType)
	}
	if assets.enqueuedAssetID != asset.ID {
		t.Fatalf("enqueued asset = %s, want %s", assets.enqueuedAssetID, asset.ID)
	}
}

func TestServiceRejectsBodyMetadataMismatch(t *testing.T) {
	actorID, uploadID := uuid.New(), uuid.New()
	tests := []struct {
		name        string
		contentType string
		size        int64
	}{
		{name: "wrong MIME", contentType: "audio/mpeg", size: 5},
		{name: "MIME parameters", contentType: "audio/flac; charset=utf-8", size: 5},
		{name: "wrong size", contentType: "audio/flac", size: 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objects := &objectStoreStub{}
			_, _, err := NewService(
				&contentStub{session: uploadSession(actorID, uploadID, uuid.New())},
				&assetStoreStub{}, objects,
			).Ingest(context.Background(), actorID, uploadID,
				test.contentType, test.size, bytes.NewBufferString("audio"))
			if !errors.Is(err, ErrInvalid) || objects.putCalls != 0 {
				t.Fatalf("Ingest() error = %v, put calls = %d", err, objects.putCalls)
			}
		})
	}
}

func TestServiceMarksSessionFailedWhenObjectStorageFails(t *testing.T) {
	actorID, uploadID := uuid.New(), uuid.New()
	contentUploads := &contentStub{session: uploadSession(actorID, uploadID, uuid.New())}
	_, _, err := NewService(contentUploads, &assetStoreStub{findErr: media.ErrNotFound},
		&objectStoreStub{putErr: errors.New("minio unavailable")}).Ingest(
		context.Background(), actorID, uploadID, "audio/flac", 5,
		bytes.NewBufferString("audio"),
	)
	if !errors.Is(err, ErrStorage) || len(contentUploads.transitions) != 2 ||
		contentUploads.transitions[1] != content.StatusFailed {
		t.Fatalf("Ingest() error = %v, transitions = %v", err, contentUploads.transitions)
	}
}

func TestServiceCompensatesWhenAssetCreationFails(t *testing.T) {
	actorID, uploadID := uuid.New(), uuid.New()
	contentUploads := &contentStub{session: uploadSession(actorID, uploadID, uuid.New())}
	objects := &objectStoreStub{}
	_, _, err := NewService(contentUploads, &assetStoreStub{
		findErr: media.ErrNotFound, createErr: errors.New("database unavailable"),
	}, objects).Ingest(context.Background(), actorID, uploadID, "audio/flac", 5,
		bytes.NewBufferString("audio"))
	if err == nil || objects.deleteCalls != 1 ||
		contentUploads.transitions[len(contentUploads.transitions)-1] != content.StatusFailed {
		t.Fatalf("Ingest() error = %v, deletes = %d, transitions = %v",
			err, objects.deleteCalls, contentUploads.transitions)
	}
}

func TestServiceReturnsExistingCompletedAssetIdempotently(t *testing.T) {
	actorID, uploadID, trackID := uuid.New(), uuid.New(), uuid.New()
	session := uploadSession(actorID, uploadID, trackID)
	session.Status = content.StatusCompleted
	existing := media.Asset{
		ID: uuid.New(), ContentTrackID: trackID, SourceUploadID: uploadID,
		OriginalObjectKey: session.ObjectKey, OriginalFilename: session.OriginalName,
		Status: media.StatusUploaded,
	}
	objects := &objectStoreStub{}
	asset, created, err := NewService(&contentStub{session: session},
		&assetStoreStub{existing: existing}, objects).Ingest(
		context.Background(), actorID, uploadID, "audio/flac", 5,
		bytes.NewBufferString("audio"))
	if err != nil || created || asset.ID != existing.ID || objects.putCalls != 0 {
		t.Fatalf("Ingest() = %+v, created %v, error %v, puts %d",
			asset, created, err, objects.putCalls)
	}
}

func TestServiceReturnsErrorWhenProcessingCannotBeEnqueued(t *testing.T) {
	actorID, uploadID := uuid.New(), uuid.New()
	_, _, err := NewService(
		&contentStub{session: uploadSession(actorID, uploadID, uuid.New())},
		&assetStoreStub{findErr: media.ErrNotFound, enqueueErr: errors.New("database unavailable")},
		&objectStoreStub{},
	).Ingest(context.Background(), actorID, uploadID, "audio/flac", 5,
		bytes.NewBufferString("audio"))
	if err == nil || !strings.Contains(err.Error(), "enqueue media processing") {
		t.Fatalf("Ingest() error = %v", err)
	}
}

func TestServiceRejectsConcurrentUploadWithoutAsset(t *testing.T) {
	actorID, uploadID := uuid.New(), uuid.New()
	session := uploadSession(actorID, uploadID, uuid.New())
	session.Status = content.StatusUploading
	_, _, err := NewService(&contentStub{session: session},
		&assetStoreStub{findErr: media.ErrNotFound}, &objectStoreStub{}).Ingest(
		context.Background(), actorID, uploadID, "audio/flac", 5,
		bytes.NewBufferString("audio"))
	if !errors.Is(err, ErrInProgress) {
		t.Fatalf("Ingest() error = %v", err)
	}
}

func uploadSession(actorID, uploadID, trackID uuid.UUID) content.Session {
	return content.Session{
		ID: uploadID, ReleaseTrackID: trackID, RequestedByUserID: actorID,
		ObjectKey:    "creator-content/" + uploadID.String() + "/source.flac",
		OriginalName: "source.flac", MIMEType: "audio/flac", SizeBytes: 5,
		Status: content.StatusPending,
	}
}

type contentStub struct {
	session       content.Session
	sessionErr    error
	transitionErr error
	transitions   []content.Status
}

func (stub *contentStub) Session(
	context.Context, uuid.UUID, uuid.UUID,
) (content.Session, error) {
	return stub.session, stub.sessionErr
}

func (stub *contentStub) Transition(
	_ context.Context,
	_ uuid.UUID,
	target content.Status,
) (content.Session, error) {
	stub.transitions = append(stub.transitions, target)
	if stub.transitionErr != nil {
		return content.Session{}, stub.transitionErr
	}
	stub.session.Status = target
	return stub.session, nil
}

type assetStoreStub struct {
	existing        media.Asset
	findErr         error
	createErr       error
	enqueueErr      error
	enqueuedAssetID uuid.UUID
}

func (stub *assetStoreStub) EnqueueProcessing(
	_ context.Context,
	assetID uuid.UUID,
) error {
	stub.enqueuedAssetID = assetID
	return stub.enqueueErr
}

func (stub *assetStoreStub) AssetBySourceUpload(
	context.Context, uuid.UUID,
) (media.Asset, error) {
	return stub.existing, stub.findErr
}

func (stub *assetStoreStub) CreateFromUpload(
	_ context.Context,
	asset media.Asset,
) (media.Asset, bool, error) {
	if stub.createErr != nil {
		return media.Asset{}, false, stub.createErr
	}
	return asset, true, nil
}

type objectStoreStub struct {
	key         string
	contentType string
	data        []byte
	putErr      error
	deleteErr   error
	putCalls    int
	deleteCalls int
}

func (stub *objectStoreStub) Put(
	_ context.Context,
	key string,
	reader io.Reader,
	_ int64,
	contentType string,
) error {
	stub.putCalls++
	stub.key = key
	stub.contentType = contentType
	stub.data, _ = io.ReadAll(reader)
	return stub.putErr
}

func (stub *objectStoreStub) Delete(context.Context, string) error {
	stub.deleteCalls++
	return stub.deleteErr
}
