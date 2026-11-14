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
	issues, err := service.contentIssues(ctx, candidate, false)
	if err != nil {
		return release.Release{}, err
	}
	if len(issues) > 0 {
		readiness := &ReadinessError{Reason: issues[0].Message}
		if issues[0].TrackID != nil {
			readiness.TrackID = *issues[0].TrackID
		}
		return release.Release{}, readiness
	}
	result, err := service.store.SubmitRelease(ctx, releaseID)
	if err != nil {
		return release.Release{}, fmt.Errorf("submit release for moderation: %w", err)
	}
	return result, nil
}

// contentIssues is shared by the read-only readiness report and Submit. Submit
// stops on the first blocker, retaining its existing error semantics. Readiness
// collects independent problems across tracks for actionable UI feedback.
func (service *Service) contentIssues(
	ctx context.Context, candidate Candidate, collectAll bool,
) ([]ReadinessIssue, error) {
	issues := make([]ReadinessIssue, 0)
	if strings.TrimSpace(candidate.Release.Title) == "" ||
		candidate.Release.ReleaseDate.IsZero() || !validReleaseType(candidate.Release.Type) {
		issues = append(issues, ReadinessIssue{
			Code: "metadata_incomplete", Message: "release title, type and date are required",
		})
		if !collectAll {
			return issues, nil
		}
	}
	if candidate.Release.CoverObjectKey == nil ||
		strings.TrimSpace(*candidate.Release.CoverObjectKey) == "" {
		issues = append(issues, ReadinessIssue{
			Code: "cover_missing", Message: "cover artwork is required",
		})
		if !collectAll {
			return issues, nil
		}
	}
	if len(candidate.Tracks) == 0 {
		return append(issues, ReadinessIssue{
			Code: "tracks_missing", Message: "at least one track is required",
		}), nil
	}
	for _, track := range candidate.Tracks {
		trackIssues, err := service.inspectTrack(ctx, track, collectAll)
		if err != nil {
			return nil, err
		}
		issues = append(issues, trackIssues...)
		if !collectAll && len(issues) > 0 {
			return issues, nil
		}
	}
	return issues, nil
}

func (service *Service) inspectTrack(
	ctx context.Context, track Track, collectAll bool,
) ([]ReadinessIssue, error) {
	issues := make([]ReadinessIssue, 0)
	add := func(code, message string) {
		trackID := track.ID
		issues = append(issues, ReadinessIssue{Code: code, Message: message, TrackID: &trackID})
	}
	if !track.HasWorldwideStreaming {
		add("streaming_rights_missing", "worldwide streaming rights are required")
	}
	if !track.HasWorldwideDistribution {
		add("distribution_rights_missing", "worldwide distribution rights are required")
	}
	if len(issues) > 0 && !collectAll {
		return issues, nil
	}
	var asset contentmedia.Asset
	var err error
	if track.CompletedUploadID != nil && *track.CompletedUploadID != uuid.Nil {
		asset, err = service.media.AssetByUpload(ctx, *track.CompletedUploadID)
	} else {
		asset, err = service.media.AssetByTrack(ctx, track.ID)
	}
	if errors.Is(err, contentmedia.ErrNotFound) {
		add("media_missing", "processed media asset is required")
		return issues, nil
	}
	if errors.Is(err, contentmedia.ErrUnavailable) {
		return nil, fmt.Errorf("check track %s media: %w", track.ID, ErrMediaUnavailable)
	}
	if err != nil {
		return nil, fmt.Errorf("check track %s media: %w: %v", track.ID, ErrMediaUnavailable, err)
	}
	if asset.ContentTrackID != track.ID ||
		(track.CompletedUploadID != nil && asset.SourceUploadID != *track.CompletedUploadID) ||
		(track.CompletedUploadID == nil && asset.SourceUploadID != uuid.Nil) {
		return nil, fmt.Errorf("track %s: %w", track.ID, ErrInconsistentMedia)
	}
	if asset.Status != contentmedia.StatusReady {
		add("media_not_ready", "media processing must be ready")
	}
	return issues, nil
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
