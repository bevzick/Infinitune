package publication

import (
	"context"
	"errors"
	"testing"
	"time"

	contentcatalog "github.com/bevzick/Infinitune/services/content-service/internal/catalog"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	contentmedia "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func TestServicePublishesApprovedAlbum(t *testing.T) {
	actorID, creatorID, releaseID := uuid.New(), uuid.New(), uuid.New()
	trackID, uploadID, assetID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	store := &storeStub{candidate: Candidate{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Approved Album",
			Type: release.TypeAlbum, ReleaseDate: time.Date(2027, 3, 4, 0, 0, 0, 0, time.UTC),
			Status: release.StatusApproved,
		},
		CreatorName: "Artist",
		Tracks: []Track{{Track: release.Track{
			ID: trackID, ReleaseID: releaseID, Title: "Track",
			TrackNumber: 1, DiscNumber: 1,
		}, CompletedUploadID: &uploadID}},
	}, result: release.Release{ID: releaseID, Status: release.StatusPublished}}
	media := &mediaStub{assets: map[uuid.UUID]contentmedia.Asset{
		uploadID: {
			ID: assetID, SourceUploadID: uploadID, ContentTrackID: trackID,
			DurationMS: 180000, Status: contentmedia.StatusReady,
		},
	}}
	catalog := &catalogStub{publishedAt: now}
	result, err := NewService(store, &accessStub{decision: creator.AccessDecision{Allowed: true}},
		media, catalog).Publish(context.Background(), actorID, releaseID)
	if err != nil || result.Status != release.StatusPublished || store.markCalls != 1 {
		t.Fatalf("Publish() = %+v, %v; mark calls = %d", result, err, store.markCalls)
	}
	wantArtistID := stableID(artistNamespace, creatorID)
	wantAlbumID := stableID(albumNamespace, releaseID)
	wantTrackID := stableID(trackNamespace, trackID)
	if catalog.input.ArtistID != wantArtistID || catalog.input.AlbumID == nil ||
		*catalog.input.AlbumID != wantAlbumID || len(catalog.input.Tracks) != 1 ||
		catalog.input.Tracks[0].ID != wantTrackID ||
		catalog.input.Tracks[0].MediaAssetID != assetID ||
		store.record.AlbumID == nil || *store.record.AlbumID != wantAlbumID ||
		store.record.PublishedAt != now {
		t.Fatalf("catalog input = %+v; record = %+v", catalog.input, store.record)
	}
	if access := catalog.input.Tracks[0]; access.DurationMS != 180000 {
		t.Fatalf("published track = %+v", access)
	}
}

func TestServicePublishesSingleWithoutAlbum(t *testing.T) {
	releaseID, trackID, uploadID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{candidate: Candidate{
		Release: release.Release{
			ID: releaseID, CreatorID: uuid.New(), Title: "Single",
			Type: release.TypeSingle, ReleaseDate: time.Now(), Status: release.StatusApproved,
		}, CreatorName: "Solo",
		Tracks: []Track{{Track: release.Track{
			ID: trackID, ReleaseID: releaseID, Title: "Single", TrackNumber: 1, DiscNumber: 1,
		}, CompletedUploadID: &uploadID}},
	}, result: release.Release{ID: releaseID, Status: release.StatusPublished}}
	catalog := &catalogStub{publishedAt: time.Now().UTC()}
	_, err := NewService(store, &accessStub{decision: creator.AccessDecision{Allowed: true}},
		&mediaStub{assets: map[uuid.UUID]contentmedia.Asset{uploadID: {
			ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: trackID,
			DurationMS: 1000, Status: contentmedia.StatusReady,
		}}}, catalog).Publish(context.Background(), uuid.New(), releaseID)
	if err != nil || catalog.input.AlbumID != nil || store.record.AlbumID != nil {
		t.Fatalf("Publish() error = %v; catalog = %+v; record = %+v", err, catalog.input, store.record)
	}
}

func TestServicePublishesTrackWithExternalMedia(t *testing.T) {
	releaseID, trackID, assetID := uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{candidate: Candidate{
		Release: release.Release{
			ID: releaseID, CreatorID: uuid.New(), Title: "Imported Single",
			Type: release.TypeSingle, ReleaseDate: time.Now(), Status: release.StatusApproved,
		}, CreatorName: "Artist",
		Tracks: []Track{{Track: release.Track{
			ID: trackID, ReleaseID: releaseID, Title: "Imported Track",
			TrackNumber: 1, DiscNumber: 1,
		}}},
	}, result: release.Release{ID: releaseID, Status: release.StatusPublished}}
	mediaAssets := &mediaStub{assets: map[uuid.UUID]contentmedia.Asset{trackID: {
		ID: assetID, ContentTrackID: trackID, DurationMS: 125000,
		Status: contentmedia.StatusReady,
	}}}
	catalog := &catalogStub{publishedAt: time.Now().UTC()}
	_, err := NewService(store,
		&accessStub{decision: creator.AccessDecision{Allowed: true}}, mediaAssets, catalog,
	).Publish(context.Background(), uuid.New(), releaseID)
	if err != nil || len(catalog.input.Tracks) != 1 ||
		catalog.input.Tracks[0].MediaAssetID != assetID {
		t.Fatalf("Publish() error = %v; catalog = %+v", err, catalog.input)
	}
}

func TestServiceReturnsPublishedReleaseIdempotently(t *testing.T) {
	releaseID := uuid.New()
	store := &storeStub{candidate: Candidate{Release: release.Release{
		ID: releaseID, CreatorID: uuid.New(), Status: release.StatusPublished,
	}}}
	result, err := NewService(store,
		&accessStub{decision: creator.AccessDecision{Allowed: true}}, &mediaStub{}, &catalogStub{},
	).Publish(context.Background(), uuid.New(), releaseID)
	if err != nil || result.Status != release.StatusPublished || store.markCalls != 0 {
		t.Fatalf("Publish() = %+v, %v; mark calls = %d", result, err, store.markCalls)
	}
}

func TestServiceRequiresPublisherAndApprovedStatus(t *testing.T) {
	releaseID := uuid.New()
	tests := []struct {
		name      string
		status    release.Status
		decision  creator.AccessDecision
		wantError error
	}{
		{name: "foreign user", status: release.StatusApproved, wantError: creator.ErrForbidden},
		{name: "pending review", status: release.StatusPendingReview, decision: creator.AccessDecision{Allowed: true}, wantError: ErrConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{candidate: Candidate{Release: release.Release{
				ID: releaseID, CreatorID: uuid.New(), Status: test.status,
			}}}
			catalog := &catalogStub{}
			_, err := NewService(store, &accessStub{decision: test.decision},
				&mediaStub{}, catalog,
			).Publish(context.Background(), uuid.New(), releaseID)
			if !errors.Is(err, test.wantError) || catalog.input.SourceReleaseID != uuid.Nil {
				t.Fatalf("Publish() error = %v, want %v; catalog input = %+v",
					err, test.wantError, catalog.input)
			}
		})
	}
}

func TestServiceRejectsUnavailablePublicationDependencies(t *testing.T) {
	releaseID, trackID, uploadID := uuid.New(), uuid.New(), uuid.New()
	base := Candidate{
		Release: release.Release{
			ID: releaseID, CreatorID: uuid.New(), Type: release.TypeSingle,
			Status: release.StatusApproved, ReleaseDate: time.Now(),
		}, CreatorName: "Artist",
		Tracks: []Track{{Track: release.Track{
			ID: trackID, TrackNumber: 1, DiscNumber: 1, Title: "Track",
		}, CompletedUploadID: &uploadID}},
	}
	tests := []struct {
		name      string
		media     MediaAssets
		catalog   Catalog
		wantError error
	}{
		{name: "media", media: &mediaStub{err: contentmedia.ErrUnavailable}, catalog: &catalogStub{}, wantError: ErrMediaUnavailable},
		{name: "catalog", media: &mediaStub{assets: map[uuid.UUID]contentmedia.Asset{uploadID: {
			ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: trackID,
			DurationMS: 1000, Status: contentmedia.StatusReady,
		}}}, catalog: &catalogStub{err: contentcatalog.ErrUnavailable}, wantError: ErrCatalogUnavailable},
		{name: "catalog approval pending", media: &mediaStub{assets: map[uuid.UUID]contentmedia.Asset{uploadID: {
			ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: trackID,
			DurationMS: 1000, Status: contentmedia.StatusReady,
		}}}, catalog: &catalogStub{err: contentcatalog.ErrNotApproved}, wantError: ErrNotReady},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{candidate: base}
			_, err := NewService(store,
				&accessStub{decision: creator.AccessDecision{Allowed: true}}, test.media, test.catalog,
			).Publish(context.Background(), uuid.New(), releaseID)
			if !errors.Is(err, test.wantError) || store.markCalls != 0 {
				t.Fatalf("Publish() error = %v, want %v; mark calls = %d", err, test.wantError, store.markCalls)
			}
		})
	}
}

type storeStub struct {
	candidate Candidate
	result    release.Release
	err       error
	record    Record
	markCalls int
}

func (stub *storeStub) PublicationCandidate(context.Context, uuid.UUID) (Candidate, error) {
	return stub.candidate, stub.err
}

func (stub *storeStub) MarkPublished(_ context.Context, record Record) (release.Release, error) {
	stub.markCalls++
	stub.record = record
	return stub.result, stub.err
}

type accessStub struct {
	decision creator.AccessDecision
	err      error
}

func (stub *accessStub) Check(
	context.Context, uuid.UUID, uuid.UUID, creator.Permission,
) (creator.AccessDecision, error) {
	return stub.decision, stub.err
}

type mediaStub struct {
	assets map[uuid.UUID]contentmedia.Asset
	err    error
}

func (stub *mediaStub) AssetByUpload(_ context.Context, id uuid.UUID) (contentmedia.Asset, error) {
	return stub.assets[id], stub.err
}

func (stub *mediaStub) AssetByTrack(_ context.Context, id uuid.UUID) (contentmedia.Asset, error) {
	return stub.assets[id], stub.err
}

type catalogStub struct {
	input       contentcatalog.PublicationInput
	publishedAt time.Time
	err         error
}

func (stub *catalogStub) Publish(
	_ context.Context,
	input contentcatalog.PublicationInput,
) (contentcatalog.Publication, error) {
	stub.input = input
	if stub.err != nil {
		return contentcatalog.Publication{}, stub.err
	}
	tracks := make([]contentcatalog.PublishedTrack, 0, len(input.Tracks))
	for _, track := range input.Tracks {
		tracks = append(tracks, contentcatalog.PublishedTrack{
			ContentTrackID: track.ContentTrackID, CatalogTrackID: track.ID,
		})
	}
	return contentcatalog.Publication{
		ArtistID: input.ArtistID, AlbumID: input.AlbumID,
		Tracks: tracks, PublishedAt: stub.publishedAt,
	}, nil
}

var _ Store = (*storeStub)(nil)
var _ AccessChecker = (*accessStub)(nil)
var _ MediaAssets = (*mediaStub)(nil)
var _ Catalog = (*catalogStub)(nil)
