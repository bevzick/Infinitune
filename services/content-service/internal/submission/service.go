package submission

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	contentmedia "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

type Store interface {
	SubmissionCandidate(context.Context, uuid.UUID) (Candidate, error)
	SubmitRelease(context.Context, uuid.UUID) (release.Release, error)
}

type AccessChecker interface {
	Check(
		context.Context, uuid.UUID, uuid.UUID, creator.Permission,
	) (creator.AccessDecision, error)
}

type MediaAssets interface {
	AssetByUpload(context.Context, uuid.UUID) (contentmedia.Asset, error)
	AssetByTrack(context.Context, uuid.UUID) (contentmedia.Asset, error)
}

type Service struct {
	store  Store
	access AccessChecker
	media  MediaAssets
}

func NewService(store Store, access AccessChecker, media MediaAssets) *Service {
	return &Service{store: store, access: access, media: media}
}

func (service *Service) Submit(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (release.Release, error) {
	if releaseID == uuid.Nil {
		return release.Release{}, &ValidationError{
			Field: "release_id", Reason: "must be a UUID",
		}
	}
	candidate, err := service.store.SubmissionCandidate(ctx, releaseID)
	if err != nil {
		return release.Release{}, fmt.Errorf("read release submission candidate: %w", err)
	}
	if err = service.authorize(ctx, actorID, candidate.Release.CreatorID); err != nil {
		return release.Release{}, fmt.Errorf("authorize release submission: %w", err)
	}
	if candidate.Release.Status == release.StatusPendingReview {
		return candidate.Release, nil
	}
	if candidate.Release.Status != release.StatusDraft &&
		candidate.Release.Status != release.StatusRejected &&
		candidate.Release.Status != release.StatusChangesRequested {
		return release.Release{}, ErrConflict
	}
	if candidate.Release.CoverObjectKey == nil ||
		strings.TrimSpace(*candidate.Release.CoverObjectKey) == "" {
		return release.Release{}, &ReadinessError{Reason: "cover artwork is required"}
	}
	if len(candidate.Tracks) == 0 {
		return release.Release{}, &ReadinessError{Reason: "at least one track is required"}
	}
	for _, track := range candidate.Tracks {
		if err = service.checkTrack(ctx, track); err != nil {
			return release.Release{}, err
		}
	}
	result, err := service.store.SubmitRelease(ctx, releaseID)
	if err != nil {
		return release.Release{}, fmt.Errorf("submit release for moderation: %w", err)
	}
	return result, nil
}

func (service *Service) checkTrack(ctx context.Context, track Track) error {
	if !track.HasWorldwideStreaming {
		return &ReadinessError{TrackID: track.ID, Reason: "worldwide streaming rights are required"}
	}
	if !track.HasWorldwideDistribution {
		return &ReadinessError{TrackID: track.ID, Reason: "worldwide distribution rights are required"}
	}
	var asset contentmedia.Asset
	var err error
	if track.CompletedUploadID != nil && *track.CompletedUploadID != uuid.Nil {
		asset, err = service.media.AssetByUpload(ctx, *track.CompletedUploadID)
	} else {
		asset, err = service.media.AssetByTrack(ctx, track.ID)
	}
	if errors.Is(err, contentmedia.ErrNotFound) {
		return &ReadinessError{TrackID: track.ID, Reason: "processed media asset is required"}
	}
	if errors.Is(err, contentmedia.ErrUnavailable) {
		return fmt.Errorf("check track %s media: %w", track.ID, ErrMediaUnavailable)
	}
	if err != nil {
		return fmt.Errorf("check track %s media: %w: %v", track.ID, ErrMediaUnavailable, err)
	}
	if asset.ContentTrackID != track.ID ||
		(track.CompletedUploadID != nil && asset.SourceUploadID != *track.CompletedUploadID) ||
		(track.CompletedUploadID == nil && asset.SourceUploadID != uuid.Nil) {
		return fmt.Errorf("track %s: %w", track.ID, ErrInconsistentMedia)
	}
	if asset.Status != contentmedia.StatusReady {
		return &ReadinessError{TrackID: track.ID, Reason: "media processing must be ready"}
	}
	return nil
}

func (service *Service) authorize(
	ctx context.Context,
	actorID uuid.UUID,
	creatorID uuid.UUID,
) error {
	if actorID == uuid.Nil {
		return creator.ErrForbidden
	}
	decision, err := service.access.Check(
		ctx, actorID, creatorID, creator.PermissionContentPublish,
	)
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
