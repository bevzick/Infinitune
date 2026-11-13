package publication

import (
	"context"
	"errors"
	"fmt"
	"math"

	contentcatalog "github.com/bevzick/Infinitune/services/content-service/internal/catalog"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	contentmedia "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

var (
	artistNamespace = uuid.MustParse("bb6b7b23-c244-49ad-8747-09c37f47bdec")
	albumNamespace  = uuid.MustParse("66fa1152-ff54-457f-8d1c-9378e4f95fd9")
	trackNamespace  = uuid.MustParse("91325f35-e30e-497a-bb75-f9f9817f07a1")
)

type Store interface {
	PublicationCandidate(context.Context, uuid.UUID) (Candidate, error)
	MarkPublished(context.Context, Record) (release.Release, error)
}

type AccessChecker interface {
	Check(context.Context, uuid.UUID, uuid.UUID, creator.Permission) (creator.AccessDecision, error)
}

type MediaAssets interface {
	AssetByUpload(context.Context, uuid.UUID) (contentmedia.Asset, error)
	AssetByTrack(context.Context, uuid.UUID) (contentmedia.Asset, error)
}

type Catalog interface {
	Publish(context.Context, contentcatalog.PublicationInput) (contentcatalog.Publication, error)
}

type Service struct {
	store   Store
	access  AccessChecker
	media   MediaAssets
	catalog Catalog
}

func NewService(store Store, access AccessChecker, media MediaAssets, catalog Catalog) *Service {
	return &Service{store: store, access: access, media: media, catalog: catalog}
}

func (service *Service) Publish(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (release.Release, error) {
	if releaseID == uuid.Nil {
		return release.Release{}, &ValidationError{Field: "release_id", Reason: "must be a UUID"}
	}
	candidate, err := service.store.PublicationCandidate(ctx, releaseID)
	if err != nil {
		return release.Release{}, fmt.Errorf("read release publication candidate: %w", err)
	}
	if err = service.authorize(ctx, actorID, candidate.Release.CreatorID); err != nil {
		return release.Release{}, fmt.Errorf("authorize release publication: %w", err)
	}
	if candidate.Release.Status == release.StatusPublished {
		return candidate.Release, nil
	}
	if candidate.Release.Status != release.StatusApproved {
		return release.Release{}, ErrConflict
	}
	if len(candidate.Tracks) == 0 {
		return release.Release{}, &ReadinessError{Reason: "at least one track is required"}
	}
	input := contentcatalog.PublicationInput{
		SourceReleaseID: candidate.Release.ID,
		ArtistID:        stableID(artistNamespace, candidate.Release.CreatorID),
		ArtistName:      candidate.CreatorName, Title: candidate.Release.Title,
		Type: candidate.Release.Type, ReleaseDate: candidate.Release.ReleaseDate,
		Tracks: make([]contentcatalog.TrackInput, 0, len(candidate.Tracks)),
	}
	if candidate.Release.Type != release.TypeSingle {
		albumID := stableID(albumNamespace, candidate.Release.ID)
		input.AlbumID = &albumID
	}
	for _, track := range candidate.Tracks {
		mapped, mapErr := service.publicationTrack(ctx, track)
		if mapErr != nil {
			return release.Release{}, mapErr
		}
		input.Tracks = append(input.Tracks, mapped)
	}
	published, err := service.catalog.Publish(ctx, input)
	if errors.Is(err, contentcatalog.ErrUnavailable) {
		return release.Release{}, ErrCatalogUnavailable
	}
	if errors.Is(err, contentcatalog.ErrConflict) {
		return release.Release{}, ErrConflict
	}
	if errors.Is(err, contentcatalog.ErrInvalid) {
		return release.Release{}, ErrInconsistentCatalog
	}
	if errors.Is(err, contentcatalog.ErrNotApproved) {
		return release.Release{}, &ReadinessError{Reason: "catalog has not observed moderation approval yet"}
	}
	if err != nil {
		return release.Release{}, fmt.Errorf("publish release to catalog: %w", err)
	}
	if !publicationMatches(input, published) {
		return release.Release{}, ErrInconsistentCatalog
	}
	result, err := service.store.MarkPublished(ctx, Record{
		ReleaseID: releaseID, AlbumID: published.AlbumID,
		Tracks: published.Tracks, PublishedAt: published.PublishedAt,
	})
	if err != nil {
		return release.Release{}, fmt.Errorf("record catalog publication: %w", err)
	}
	return result, nil
}

func (service *Service) publicationTrack(
	ctx context.Context,
	track Track,
) (contentcatalog.TrackInput, error) {
	var asset contentmedia.Asset
	var err error
	if track.CompletedUploadID != nil && *track.CompletedUploadID != uuid.Nil {
		asset, err = service.media.AssetByUpload(ctx, *track.CompletedUploadID)
	} else {
		asset, err = service.media.AssetByTrack(ctx, track.ID)
	}
	if errors.Is(err, contentmedia.ErrNotFound) {
		return contentcatalog.TrackInput{}, &ReadinessError{
			TrackID: track.ID, Reason: "processed media asset is required",
		}
	}
	if errors.Is(err, contentmedia.ErrUnavailable) {
		return contentcatalog.TrackInput{}, ErrMediaUnavailable
	}
	if errors.Is(err, contentmedia.ErrInvalid) {
		return contentcatalog.TrackInput{}, ErrInconsistentMedia
	}
	if err != nil {
		return contentcatalog.TrackInput{}, fmt.Errorf("read track %s media: %w", track.ID, err)
	}
	if track.TrackNumber <= 0 || track.TrackNumber > math.MaxInt16 ||
		track.DiscNumber <= 0 || track.DiscNumber > math.MaxInt16 {
		return contentcatalog.TrackInput{}, &ReadinessError{
			TrackID: track.ID, Reason: "track and disc numbers must fit the catalog range",
		}
	}
	if asset.ContentTrackID != track.ID ||
		(track.CompletedUploadID != nil && asset.SourceUploadID != *track.CompletedUploadID) ||
		(track.CompletedUploadID == nil && asset.SourceUploadID != uuid.Nil) {
		return contentcatalog.TrackInput{}, ErrInconsistentMedia
	}
	if asset.Status != contentmedia.StatusReady || asset.DurationMS <= 0 {
		return contentcatalog.TrackInput{}, &ReadinessError{
			TrackID: track.ID, Reason: "processed media with duration is required",
		}
	}
	return contentcatalog.TrackInput{
		ID: stableID(trackNamespace, track.ID), ContentTrackID: track.ID,
		Title: track.Title, DurationMS: asset.DurationMS,
		TrackNumber: track.TrackNumber, DiscNumber: track.DiscNumber,
		Explicit: track.Explicit, MediaAssetID: asset.ID,
	}, nil
}

func (service *Service) authorize(ctx context.Context, actorID, creatorID uuid.UUID) error {
	if actorID == uuid.Nil {
		return creator.ErrForbidden
	}
	decision, err := service.access.Check(ctx, actorID, creatorID, creator.PermissionContentPublish)
	if err != nil {
		return err
	}
	if decision.Allowed {
		return nil
	}
	if decision.Role != "" && decision.VerificationStatus != creator.VerificationVerified {
		return creator.ErrVerificationRequired
	}
	return creator.ErrForbidden
}

func publicationMatches(
	input contentcatalog.PublicationInput,
	result contentcatalog.Publication,
) bool {
	if result.ArtistID != input.ArtistID || !sameID(result.AlbumID, input.AlbumID) ||
		result.PublishedAt.IsZero() || len(result.Tracks) != len(input.Tracks) {
		return false
	}
	expected := make(map[uuid.UUID]uuid.UUID, len(input.Tracks))
	for _, track := range input.Tracks {
		expected[track.ContentTrackID] = track.ID
	}
	for _, track := range result.Tracks {
		if expected[track.ContentTrackID] != track.CatalogTrackID {
			return false
		}
		delete(expected, track.ContentTrackID)
	}
	return len(expected) == 0
}

func stableID(namespace, source uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(namespace, source[:])
}

func sameID(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
