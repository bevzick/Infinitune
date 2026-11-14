package submission

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

// ReadinessIssue describes a specific blocker without exposing private media keys.
// TrackID is present only for issues associated with an individual track.
type ReadinessIssue struct {
	Code    string     `json:"code"`
	Message string     `json:"message"`
	TrackID *uuid.UUID `json:"track_id,omitempty"`
}

// ReadinessReport is a snapshot. Submission rechecks the constraints and can
// still fail when rights, media or release state change concurrently.
type ReadinessReport struct {
	ReleaseID          uuid.UUID        `json:"release_id"`
	Status             release.Status   `json:"status"`
	ReadyForSubmission bool             `json:"ready_for_submission"`
	Issues             []ReadinessIssue `json:"issues"`
}

func (service *Service) Readiness(
	ctx context.Context,
	actorID uuid.UUID,
	releaseID uuid.UUID,
) (ReadinessReport, error) {
	if releaseID == uuid.Nil {
		return ReadinessReport{}, &ValidationError{Field: "release_id", Reason: "must be a UUID"}
	}
	candidate, err := service.store.SubmissionCandidate(ctx, releaseID)
	if err != nil {
		return ReadinessReport{}, fmt.Errorf("read release readiness candidate: %w", err)
	}
	if actorID == uuid.Nil {
		return ReadinessReport{}, creator.ErrForbidden
	}
	// Account membership is required even to see which assets or rights are missing.
	access, err := service.access.Check(
		ctx, actorID, candidate.Release.CreatorID, creator.PermissionAccountRead,
	)
	if err != nil {
		return ReadinessReport{}, fmt.Errorf("authorize release readiness: %w", err)
	}
	if !access.Allowed {
		return ReadinessReport{}, creator.ErrForbidden
	}
	publisher, err := service.access.Check(
		ctx, actorID, candidate.Release.CreatorID, creator.PermissionContentPublish,
	)
	if err != nil {
		return ReadinessReport{}, fmt.Errorf("check release publishing permission: %w", err)
	}

	report := ReadinessReport{
		ReleaseID: candidate.Release.ID, Status: candidate.Release.Status,
		Issues: make([]ReadinessIssue, 0),
	}
	if publisher.VerificationStatus != creator.VerificationVerified {
		report.Issues = append(report.Issues, ReadinessIssue{
			Code: "creator_unverified", Message: "creator account must be verified",
		})
	}
	if !creator.HasPermission(publisher.Role, creator.PermissionContentPublish) {
		report.Issues = append(report.Issues, ReadinessIssue{
			Code: "publish_permission_required", Message: "creator publishing permission is required",
		})
	}
	if !submittableStatus(candidate.Release.Status) {
		report.Issues = append(report.Issues, ReadinessIssue{
			Code: "release_status_ineligible", Message: "release cannot be submitted in its current status",
		})
		return report, nil
	}

	issues, err := service.contentIssues(ctx, candidate, true)
	if err != nil {
		return ReadinessReport{}, err
	}
	report.Issues = append(report.Issues, issues...)
	report.ReadyForSubmission = len(report.Issues) == 0
	return report, nil
}

func submittableStatus(status release.Status) bool {
	return status == release.StatusDraft || status == release.StatusRejected ||
		status == release.StatusChangesRequested
}

func validReleaseType(value release.Type) bool {
	switch value {
	case release.TypeSingle, release.TypeEP, release.TypeAlbum, release.TypeCompilation:
		return true
	default:
		return false
	}
}
