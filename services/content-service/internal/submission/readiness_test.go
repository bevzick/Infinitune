package submission

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	contentmedia "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func readyCandidate() (Candidate, uuid.UUID) {
	trackID, uploadID := uuid.New(), uuid.New()
	cover := "creator-content/cover.png"
	return Candidate{
		Release: release.Release{
			ID: uuid.New(), CreatorID: uuid.New(), Status: release.StatusDraft,
			Title: "Night Signal", Type: release.TypeSingle,
			ReleaseDate:    time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC),
			CoverObjectKey: &cover,
		},
		Tracks: []Track{{
			ID: trackID, CompletedUploadID: &uploadID,
			HasWorldwideStreaming: true, HasWorldwideDistribution: true,
		}},
	}, uploadID
}

func readyMedia(candidate Candidate, uploadID uuid.UUID) *mediaStub {
	return &mediaStub{asset: contentmedia.Asset{
		ID: uuid.New(), ContentTrackID: candidate.Tracks[0].ID,
		SourceUploadID: uploadID, Status: contentmedia.StatusReady,
	}}
}

func TestReadinessReportsCompleteReleaseWithoutMutatingIt(t *testing.T) {
	candidate, uploadID := readyCandidate()
	store := &storeStub{candidate: candidate}
	access := &accessStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleOwner,
		VerificationStatus: creator.VerificationVerified,
	}}
	media := readyMedia(candidate, uploadID)
	actorID := uuid.New()
	report, err := NewService(store, access, media).Readiness(
		context.Background(), actorID, candidate.Release.ID,
	)
	if err != nil || !report.ReadyForSubmission || len(report.Issues) != 0 ||
		report.ReleaseID != candidate.Release.ID || report.Status != release.StatusDraft {
		t.Fatalf("Readiness() = %+v, %v", report, err)
	}
	if store.submitCalls != 0 || media.uploadCalls != 1 ||
		access.actorID != actorID || access.permission != creator.PermissionContentPublish {
		t.Fatalf("unexpected state change: store=%d media=%d access=%+v",
			store.submitCalls, media.uploadCalls, access)
	}
}

func TestReadinessCollectsIndependentIssues(t *testing.T) {
	candidate, _ := readyCandidate()
	candidate.Release.CoverObjectKey = nil
	candidate.Release.Title = " "
	candidate.Tracks[0].HasWorldwideStreaming = false
	candidate.Tracks[0].HasWorldwideDistribution = false
	media := &mediaStub{err: contentmedia.ErrNotFound}
	report, err := NewService(&storeStub{candidate: candidate},
		&accessStub{decision: creator.AccessDecision{
			Allowed: true, Role: creator.RoleManager,
			VerificationStatus: creator.VerificationVerified,
		}}, media,
	).Readiness(context.Background(), uuid.New(), candidate.Release.ID)
	if err != nil || report.ReadyForSubmission {
		t.Fatalf("Readiness() = %+v, %v", report, err)
	}
	codes := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		codes = append(codes, issue.Code)
		if issue.Code == "streaming_rights_missing" ||
			issue.Code == "distribution_rights_missing" || issue.Code == "media_missing" {
			if issue.TrackID == nil || *issue.TrackID != candidate.Tracks[0].ID {
				t.Errorf("issue must identify affected track: %+v", issue)
			}
		}
	}
	for _, want := range []string{
		"metadata_incomplete", "cover_missing", "streaming_rights_missing",
		"distribution_rights_missing", "media_missing",
	} {
		if !slices.Contains(codes, want) {
			t.Errorf("missing issue %q in %v", want, codes)
		}
	}
}

func TestReadinessReportsNotReadyMedia(t *testing.T) {
	candidate, uploadID := readyCandidate()
	media := readyMedia(candidate, uploadID)
	media.asset.Status = contentmedia.StatusProcessing
	report, err := NewService(&storeStub{candidate: candidate},
		&accessStub{decision: creator.AccessDecision{
			Allowed: true, Role: creator.RoleOwner,
			VerificationStatus: creator.VerificationVerified,
		}}, media,
	).Readiness(context.Background(), uuid.New(), candidate.Release.ID)
	if err != nil || report.ReadyForSubmission || len(report.Issues) != 1 ||
		report.Issues[0].Code != "media_not_ready" {
		t.Fatalf("Readiness() = %+v, %v", report, err)
	}
}

func TestReadinessReportsAccountAndPermissionBlockers(t *testing.T) {
	candidate, uploadID := readyCandidate()
	access := &accessStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleEditor,
		VerificationStatus: creator.VerificationUnverified,
	}}
	report, err := NewService(&storeStub{candidate: candidate}, access,
		readyMedia(candidate, uploadID),
	).Readiness(context.Background(), uuid.New(), candidate.Release.ID)
	if err != nil || report.ReadyForSubmission || len(report.Issues) != 2 ||
		report.Issues[0].Code != "creator_unverified" ||
		report.Issues[1].Code != "publish_permission_required" {
		t.Fatalf("Readiness() = %+v, %v", report, err)
	}
}

func TestReadinessRequiresCreatorMembership(t *testing.T) {
	candidate, _ := readyCandidate()
	media := &mediaStub{}
	_, err := NewService(&storeStub{candidate: candidate},
		&accessStub{decision: creator.AccessDecision{}}, media,
	).Readiness(context.Background(), uuid.New(), candidate.Release.ID)
	if !errors.Is(err, creator.ErrForbidden) || media.uploadCalls != 0 {
		t.Fatalf("Readiness() error = %v, media calls = %d", err, media.uploadCalls)
	}
}

func TestReadinessForNonSubmittableReleaseDoesNotRequestMedia(t *testing.T) {
	candidate, _ := readyCandidate()
	candidate.Release.Status = release.StatusPublished
	media := &mediaStub{err: contentmedia.ErrUnavailable}
	report, err := NewService(&storeStub{candidate: candidate},
		&accessStub{decision: creator.AccessDecision{
			Allowed: true, Role: creator.RoleOwner,
			VerificationStatus: creator.VerificationVerified,
		}}, media,
	).Readiness(context.Background(), uuid.New(), candidate.Release.ID)
	if err != nil || report.ReadyForSubmission || len(report.Issues) != 1 ||
		report.Issues[0].Code != "release_status_ineligible" || media.uploadCalls != 0 {
		t.Fatalf("Readiness() = %+v, %v; media calls %d", report, err, media.uploadCalls)
	}
}

func TestReadinessReportsTransientMediaFailureAsError(t *testing.T) {
	candidate, _ := readyCandidate()
	_, err := NewService(&storeStub{candidate: candidate},
		&accessStub{decision: creator.AccessDecision{
			Allowed: true, Role: creator.RoleOwner,
			VerificationStatus: creator.VerificationVerified,
		}}, &mediaStub{err: contentmedia.ErrUnavailable},
	).Readiness(context.Background(), uuid.New(), candidate.Release.ID)
	if !errors.Is(err, ErrMediaUnavailable) {
		t.Fatalf("Readiness() error = %v", err)
	}
}

func TestReadinessRejectsInvalidReleaseID(t *testing.T) {
	_, err := NewService(&storeStub{}, &accessStub{}, &mediaStub{}).Readiness(
		context.Background(), uuid.New(), uuid.Nil,
	)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Readiness() error = %v", err)
	}
}
