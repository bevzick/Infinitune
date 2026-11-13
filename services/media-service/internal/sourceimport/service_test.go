package sourceimport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bevzick/Infinitune/services/media-service/internal/content"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	"github.com/bevzick/Infinitune/services/media-service/internal/source"
	"github.com/google/uuid"
)

func TestServiceImportsAuthorizedExternalTrack(t *testing.T) {
	actorID, trackID := uuid.New(), uuid.New()
	providers := &providerStub{}
	store := &storeStub{findErr: media.ErrNotFound}
	objects := &objectStub{}
	asset, created, err := NewService(&authorizerStub{}, providers, store, objects).Import(
		context.Background(), actorID, trackID, source.ProviderJamendo, "42",
	)
	if err != nil || !created || asset.ContentTrackID != trackID ||
		asset.Status != media.StatusUploaded || string(objects.data) != "audio" {
		t.Fatalf("Import() = %+v, created %v, error %v, data %q",
			asset, created, err, objects.data)
	}
	if store.reference.Provider != source.ProviderJamendo ||
		store.reference.ExternalID != "42" || store.reference.Metadata["title"] != "Night Signal" ||
		store.reference.Metadata["artist"] != "Luna Veil" {
		t.Fatalf("reference = %+v", store.reference)
	}
	if store.asset.OriginalObjectKey == "" || store.asset.OriginalFilename != "track.mp3" ||
		providers.openCalls != 1 {
		t.Fatalf("stored asset = %+v, opens = %d", store.asset, providers.openCalls)
	}
}

func TestServiceReturnsExistingImportWithoutDownloading(t *testing.T) {
	trackID := uuid.New()
	existing := media.Asset{ID: uuid.New(), ContentTrackID: trackID}
	providers := &providerStub{}
	asset, created, err := NewService(&authorizerStub{}, providers, &storeStub{
		existing:  existing,
		reference: source.Reference{Provider: source.ProviderJamendo, ExternalID: "42"},
	}, &objectStub{}).Import(context.Background(), uuid.New(), trackID,
		source.ProviderJamendo, "42")
	if err != nil || created || asset.ID != existing.ID || providers.resolveCalls != 0 {
		t.Fatalf("Import() = %+v, created %v, error %v, resolves %d",
			asset, created, err, providers.resolveCalls)
	}
}

func TestServiceRejectsForbiddenAndIncompatibleLicense(t *testing.T) {
	trackID := uuid.New()
	_, _, err := NewService(&authorizerStub{err: content.ErrForbidden}, &providerStub{},
		&storeStub{}, &objectStub{}).Import(context.Background(), uuid.New(), trackID,
		source.ProviderJamendo, "42")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Import(forbidden) error = %v", err)
	}
	providers := &providerStub{denyRedistribution: true}
	objects := &objectStub{}
	_, _, err = NewService(&authorizerStub{}, providers,
		&storeStub{findErr: media.ErrNotFound}, objects).Import(
		context.Background(), uuid.New(), trackID, source.ProviderJamendo, "42")
	if !errors.Is(err, ErrLicense) || providers.openCalls != 0 || objects.putCalls != 0 {
		t.Fatalf("Import(license) error = %v, opens %d, puts %d",
			err, providers.openCalls, objects.putCalls)
	}
}

func TestServiceDeletesObjectWhenDatabaseWriteFails(t *testing.T) {
	objects := &objectStub{}
	_, _, err := NewService(&authorizerStub{}, &providerStub{}, &storeStub{
		findErr: media.ErrNotFound, createErr: errors.New("database unavailable"),
	}, objects).Import(context.Background(), uuid.New(), uuid.New(), source.ProviderJamendo, "42")
	if err == nil || objects.deleteCalls != 1 {
		t.Fatalf("Import() error = %v, deletes = %d", err, objects.deleteCalls)
	}
}

type authorizerStub struct{ err error }

func (stub *authorizerStub) CanImport(context.Context, uuid.UUID, uuid.UUID) error {
	return stub.err
}

type providerStub struct {
	resolveCalls       int
	openCalls          int
	denyRedistribution bool
}

func (stub *providerStub) Resolve(
	context.Context, source.ProviderName, string,
) (source.Track, error) {
	stub.resolveCalls++
	return source.Track{
		ExternalID: "42", Title: "Night Signal", Artist: "Luna Veil",
		PageURL: "https://www.jamendo.com/track/42",
		License: source.License{
			Name: "CC BY 4.0", URL: "https://creativecommons.org/licenses/by/4.0/",
			Attribution: "Luna Veil", AllowsStreaming: true,
			AllowsRedistribution: !stub.denyRedistribution,
		},
	}, nil
}

func (stub *providerStub) Open(
	context.Context, source.ProviderName, string,
) (source.AudioStream, error) {
	stub.openCalls++
	return source.AudioStream{
		Body: io.NopCloser(bytes.NewBufferString("audio")), Filename: "track.mp3",
		MIMEType: "audio/mpeg", SizeBytes: 5,
	}, nil
}

type storeStub struct {
	existing  media.Asset
	reference source.Reference
	asset     media.Asset
	findErr   error
	createErr error
}

func (stub *storeStub) ExternalByContentTrack(
	context.Context, uuid.UUID,
) (media.Asset, source.Reference, error) {
	return stub.existing, stub.reference, stub.findErr
}

func (stub *storeStub) CreateExternal(
	_ context.Context,
	asset media.Asset,
	reference source.Reference,
) (media.Asset, bool, error) {
	stub.asset = asset
	stub.reference = reference
	if stub.createErr != nil {
		return media.Asset{}, false, stub.createErr
	}
	return asset, true, nil
}

type objectStub struct {
	data        []byte
	putCalls    int
	deleteCalls int
}

func (stub *objectStub) Put(
	_ context.Context, _ string, reader io.Reader, _ int64, _ string,
) error {
	stub.putCalls++
	stub.data, _ = io.ReadAll(reader)
	return nil
}

func (stub *objectStub) Delete(context.Context, string) error {
	stub.deleteCalls++
	return nil
}
