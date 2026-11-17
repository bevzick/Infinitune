package sourceimport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bevzick/Infinitune/services/media-service/internal/artistimage"
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
	asset, created, err := NewService(&authorizerStub{}, providers, store, objects, nil).Import(
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
	}, &objectStub{}, nil).Import(context.Background(), uuid.New(), trackID,
		source.ProviderJamendo, "42")
	if err != nil || created || asset.ID != existing.ID || providers.resolveCalls != 1 {
		t.Fatalf("Import() = %+v, created %v, error %v, resolves %d",
			asset, created, err, providers.resolveCalls)
	}
}

func TestServiceRejectsForbiddenAndIncompatibleLicense(t *testing.T) {
	trackID := uuid.New()
	_, _, err := NewService(&authorizerStub{err: content.ErrForbidden}, &providerStub{},
		&storeStub{}, &objectStub{}, nil).Import(context.Background(), uuid.New(), trackID,
		source.ProviderJamendo, "42")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Import(forbidden) error = %v", err)
	}
	providers := &providerStub{denyRedistribution: true}
	objects := &objectStub{}
	_, _, err = NewService(&authorizerStub{}, providers,
		&storeStub{findErr: media.ErrNotFound}, objects, nil).Import(
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
	}, objects, nil).Import(context.Background(), uuid.New(), uuid.New(), source.ProviderJamendo, "42")
	if err == nil || objects.deleteCalls != 1 {
		t.Fatalf("Import() error = %v, deletes = %d", err, objects.deleteCalls)
	}
}

func TestServiceImportsProviderArtworkForReleaseWithoutCover(t *testing.T) {
	releaseID := uuid.New()
	providers := &providerStub{artworkAvailable: true}
	objects := &objectStub{}
	covers := &coverStub{session: content.CoverSession{
		ID: uuid.New(), ReleaseID: releaseID, RequestedByUserID: uuid.New(),
		ObjectKey: "creator-content/creator/release/covers/import/cover.jpg",
		MIMEType:  "image/jpeg", SizeBytes: 5, Status: content.StatusPending,
	}}
	_, _, err := NewService(
		&authorizerStub{target: content.ImportTarget{ReleaseID: releaseID}},
		providers, &storeStub{findErr: media.ErrNotFound}, objects, covers,
	).Import(context.Background(), uuid.New(), uuid.New(), source.ProviderJamendo, "42")
	if err != nil || providers.artworkCalls != 1 || covers.createCalls != 1 ||
		covers.completeCalls != 1 || objects.putCalls != 2 || string(objects.data) != "image" {
		t.Fatalf("Import() error = %v; artwork opens=%d creates=%d completes=%d puts=%d data=%q",
			err, providers.artworkCalls, covers.createCalls, covers.completeCalls,
			objects.putCalls, objects.data)
	}
}

func TestServiceKeepsCreatorCoverInsteadOfProviderArtwork(t *testing.T) {
	providers := &providerStub{artworkAvailable: true}
	covers := &coverStub{}
	_, _, err := NewService(
		&authorizerStub{target: content.ImportTarget{ReleaseID: uuid.New(), HasCover: true}},
		providers, &storeStub{findErr: media.ErrNotFound}, &objectStub{}, covers,
	).Import(context.Background(), uuid.New(), uuid.New(), source.ProviderJamendo, "42")
	if err != nil || providers.artworkCalls != 0 || covers.createCalls != 0 {
		t.Fatalf("Import() error = %v; artwork opens=%d creates=%d",
			err, providers.artworkCalls, covers.createCalls)
	}
}

func TestServiceRepairsArtworkForExistingExternalImport(t *testing.T) {
	releaseID, trackID := uuid.New(), uuid.New()
	existing := media.Asset{ID: uuid.New(), ContentTrackID: trackID}
	providers := &providerStub{artworkAvailable: true}
	objects := &objectStub{}
	covers := newCoverStub(releaseID)
	asset, created, err := NewService(
		&authorizerStub{target: content.ImportTarget{ReleaseID: releaseID}}, providers,
		&storeStub{existing: existing, reference: source.Reference{
			Provider: source.ProviderJamendo, ExternalID: "42",
		}}, objects, covers,
	).Import(context.Background(), uuid.New(), trackID, source.ProviderJamendo, "42")
	if err != nil || created || asset.ID != existing.ID || providers.openCalls != 0 ||
		providers.artworkCalls != 1 || objects.putCalls != 1 || covers.completeCalls != 1 {
		t.Fatalf("Import() = %+v, created=%v, error=%v; audio=%d artwork=%d puts=%d completes=%d",
			asset, created, err, providers.openCalls, providers.artworkCalls,
			objects.putCalls, covers.completeCalls)
	}
}

func TestServiceDiscardsProviderArtworkWhenCreatorCoverWinsRace(t *testing.T) {
	releaseID := uuid.New()
	providers := &providerStub{artworkAvailable: true}
	objects := &objectStub{}
	covers := newCoverStub(releaseID)
	covers.completeErr = content.ErrConflict
	_, created, err := NewService(
		&authorizerStub{target: content.ImportTarget{ReleaseID: releaseID}}, providers,
		&storeStub{findErr: media.ErrNotFound}, objects, covers,
	).Import(context.Background(), uuid.New(), uuid.New(), source.ProviderJamendo, "42")
	if err != nil || !created || covers.completeCalls != 1 || objects.deleteCalls != 1 ||
		len(objects.deletedKeys) != 1 || objects.deletedKeys[0] != covers.session.ObjectKey {
		t.Fatalf("Import() created=%v, error=%v; completes=%d deletes=%d",
			created, err, covers.completeCalls, objects.deleteCalls)
	}
}

func TestServiceImportsProviderArtistArtworkWhenMissing(t *testing.T) {
	creatorID := uuid.New()
	providers := &providerStub{artistArtworkAvailable: true}
	images := &artistImagesStub{findErr: artistimage.ErrNotFound}
	_, _, err := NewService(
		&authorizerStub{target: content.ImportTarget{
			ReleaseID: uuid.New(), CreatorID: creatorID, HasCover: true,
		}},
		providers, &storeStub{findErr: media.ErrNotFound}, &objectStub{}, nil, images,
	).Import(context.Background(), uuid.New(), uuid.New(), source.ProviderJamendo, "42")
	if err != nil || providers.artistArtworkCalls != 1 || images.importCalls != 1 ||
		images.creatorID != creatorID || string(images.data) != "artist-image" {
		t.Fatalf("Import() error=%v; artist opens=%d image imports=%d creator=%s data=%q",
			err, providers.artistArtworkCalls, images.importCalls, images.creatorID, images.data)
	}
}

func TestServiceKeepsExistingArtistImage(t *testing.T) {
	creatorID := uuid.New()
	providers := &providerStub{artistArtworkAvailable: true}
	images := &artistImagesStub{found: artistimage.Image{
		CatalogArtistID: artistimage.CatalogArtistID(creatorID),
	}}
	_, _, err := NewService(
		&authorizerStub{target: content.ImportTarget{
			ReleaseID: uuid.New(), CreatorID: creatorID, HasCover: true,
		}},
		providers, &storeStub{findErr: media.ErrNotFound}, &objectStub{}, nil, images,
	).Import(context.Background(), uuid.New(), uuid.New(), source.ProviderJamendo, "42")
	if err != nil || providers.artistArtworkCalls != 0 || images.importCalls != 0 {
		t.Fatalf("Import() error=%v; artist opens=%d image imports=%d",
			err, providers.artistArtworkCalls, images.importCalls)
	}
}

func newCoverStub(releaseID uuid.UUID) *coverStub {
	return &coverStub{session: content.CoverSession{
		ID: uuid.New(), ReleaseID: releaseID, RequestedByUserID: uuid.New(),
		ObjectKey: "creator-content/creator/release/covers/import/cover.jpg",
		MIMEType:  "image/jpeg", SizeBytes: 5, Status: content.StatusPending,
	}}
}

type authorizerStub struct {
	target content.ImportTarget
	err    error
}

func (stub *authorizerStub) CanImport(context.Context, uuid.UUID, uuid.UUID) (content.ImportTarget, error) {
	if stub.target.ReleaseID == uuid.Nil {
		stub.target.ReleaseID = uuid.New()
	}
	return stub.target, stub.err
}

type providerStub struct {
	resolveCalls           int
	openCalls              int
	artworkCalls           int
	artistArtworkCalls     int
	artworkAvailable       bool
	artistArtworkAvailable bool
	denyRedistribution     bool
}

func (stub *providerStub) Resolve(
	context.Context, source.ProviderName, string,
) (source.Track, error) {
	stub.resolveCalls++
	track := source.Track{
		ExternalID: "42", Title: "Night Signal", Artist: "Luna Veil",
		PageURL: "https://www.jamendo.com/track/42",
		License: source.License{
			Name: "CC BY 4.0", URL: "https://creativecommons.org/licenses/by/4.0/",
			Attribution: "Luna Veil", AllowsStreaming: true,
			AllowsRedistribution: !stub.denyRedistribution,
		},
	}
	if stub.artworkAvailable {
		track.ArtworkURL = "https://usercontent.jamendo.com/cover.jpg"
	}
	return track, nil
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

func (stub *providerStub) OpenArtwork(
	context.Context, source.ProviderName, string,
) (source.ArtworkStream, error) {
	stub.artworkCalls++
	if !stub.artworkAvailable {
		return source.ArtworkStream{}, source.ErrNotFound
	}
	return source.ArtworkStream{
		Body: io.NopCloser(bytes.NewBufferString("image")), Filename: "cover.jpg",
		MIMEType: "image/jpeg", SizeBytes: 5,
	}, nil
}

func (stub *providerStub) OpenArtistArtwork(
	context.Context, source.ProviderName, string,
) (source.ArtworkStream, error) {
	stub.artistArtworkCalls++
	if !stub.artistArtworkAvailable {
		return source.ArtworkStream{}, source.ErrNotFound
	}
	return source.ArtworkStream{
		Body: io.NopCloser(bytes.NewBufferString("artist-image")), Filename: "artist.jpg",
		MIMEType: "image/jpeg", SizeBytes: 12,
	}, nil
}

type artistImagesStub struct {
	found       artistimage.Image
	findErr     error
	creatorID   uuid.UUID
	data        []byte
	importCalls int
}

func (stub *artistImagesStub) Find(
	context.Context, uuid.UUID,
) (artistimage.Image, error) {
	return stub.found, stub.findErr
}

func (stub *artistImagesStub) Import(
	_ context.Context,
	_ uuid.UUID,
	creatorID uuid.UUID,
	_ string,
	_ int64,
	body io.Reader,
) (artistimage.Image, bool, error) {
	stub.importCalls++
	stub.creatorID = creatorID
	stub.data, _ = io.ReadAll(body)
	return artistimage.Image{}, true, nil
}

type coverStub struct {
	session       content.CoverSession
	createCalls   int
	completeCalls int
	completeErr   error
}

func (stub *coverStub) CreateImported(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, int64,
) (content.CoverSession, error) {
	stub.createCalls++
	return stub.session, nil
}

func (stub *coverStub) Transition(
	_ context.Context, _ uuid.UUID, status content.Status,
) (content.CoverSession, error) {
	stub.session.Status = status
	return stub.session, nil
}

func (stub *coverStub) CompleteIfMissing(
	context.Context, uuid.UUID,
) (content.CoverSession, error) {
	stub.completeCalls++
	if stub.completeErr != nil {
		return content.CoverSession{}, stub.completeErr
	}
	stub.session.Status = content.StatusCompleted
	return stub.session, nil
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
	deletedKeys []string
}

func (stub *objectStub) Put(
	_ context.Context, _ string, reader io.Reader, _ int64, _ string,
) error {
	stub.putCalls++
	stub.data, _ = io.ReadAll(reader)
	return nil
}

func (stub *objectStub) Delete(_ context.Context, objectKey string) error {
	stub.deleteCalls++
	stub.deletedKeys = append(stub.deletedKeys, objectKey)
	return nil
}
