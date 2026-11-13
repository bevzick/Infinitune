package moderation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func TestServiceRecordsModeratorDecision(t *testing.T) {
	releaseID, reviewerID := uuid.New(), uuid.New()
	store := &storeStub{result: Review{
		ID: uuid.New(), ReleaseID: releaseID, ReviewerUserID: reviewerID,
		Decision: DecisionChangesRequested, Comment: "fix metadata",
		ReleaseStatus: release.StatusChangesRequested,
	}}
	result, err := NewService(store).Review(context.Background(), creator.Actor{
		UserID: reviewerID, Roles: []string{creator.GlobalRoleModerator},
	}, Input{
		ReleaseID: releaseID, Decision: DecisionChangesRequested,
		Comment: "  fix metadata  ",
	})
	if err != nil || result.ID != store.result.ID || store.calls != 1 {
		t.Fatalf("Review() = %+v, %v; calls = %d", result, err, store.calls)
	}
	if store.review.ID == uuid.Nil || store.review.ReleaseID != releaseID ||
		store.review.ReviewerUserID != reviewerID ||
		store.review.Decision != DecisionChangesRequested ||
		store.review.Comment != "fix metadata" {
		t.Fatalf("stored review = %+v", store.review)
	}
}

func TestServiceAllowsAdminApprovalWithoutComment(t *testing.T) {
	store := &storeStub{}
	_, err := NewService(store).Review(context.Background(), creator.Actor{
		UserID: uuid.New(), Roles: []string{creator.GlobalRoleAdmin},
	}, Input{ReleaseID: uuid.New(), Decision: DecisionApproved})
	if err != nil || store.calls != 1 {
		t.Fatalf("Review() error = %v; calls = %d", err, store.calls)
	}
}

func TestServiceRejectsUnauthorizedOrInvalidReview(t *testing.T) {
	tests := []struct {
		name      string
		actor     creator.Actor
		input     Input
		wantError error
	}{
		{name: "ordinary user", actor: creator.Actor{UserID: uuid.New()}, input: Input{ReleaseID: uuid.New(), Decision: DecisionApproved}, wantError: ErrForbidden},
		{name: "missing actor", actor: creator.Actor{Roles: []string{creator.GlobalRoleModerator}}, input: Input{ReleaseID: uuid.New(), Decision: DecisionApproved}, wantError: ErrForbidden},
		{name: "missing release", actor: moderatorActor(), input: Input{Decision: DecisionApproved}, wantError: ErrInvalid},
		{name: "invalid decision", actor: moderatorActor(), input: Input{ReleaseID: uuid.New(), Decision: "publish"}, wantError: ErrInvalid},
		{name: "missing rejection comment", actor: moderatorActor(), input: Input{ReleaseID: uuid.New(), Decision: DecisionRejected}, wantError: ErrInvalid},
		{name: "missing changes comment", actor: moderatorActor(), input: Input{ReleaseID: uuid.New(), Decision: DecisionChangesRequested}, wantError: ErrInvalid},
		{name: "long comment", actor: moderatorActor(), input: Input{ReleaseID: uuid.New(), Decision: DecisionApproved, Comment: strings.Repeat("a", maximumCommentLength+1)}, wantError: ErrInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{}
			_, err := NewService(store).Review(context.Background(), test.actor, test.input)
			if !errors.Is(err, test.wantError) || store.calls != 0 {
				t.Fatalf("Review() error = %v, want %v; calls = %d", err, test.wantError, store.calls)
			}
		})
	}
}

func TestServicePreservesStoreError(t *testing.T) {
	store := &storeStub{err: ErrSelfReview}
	_, err := NewService(store).Review(context.Background(), moderatorActor(), Input{
		ReleaseID: uuid.New(), Decision: DecisionApproved,
	})
	if !errors.Is(err, ErrSelfReview) {
		t.Fatalf("Review() error = %v", err)
	}
}

func TestServiceListsAuditForModerator(t *testing.T) {
	store := &storeStub{page: Page{Items: []Review{{ID: uuid.New()}}, Total: 1}}
	page, err := NewService(store).List(context.Background(), moderatorActor(), PageRequest{})
	if err != nil || page.Total != 1 || store.pageRequest.Limit != defaultPageLimit {
		t.Fatalf("List() = %+v, %v; request = %+v", page, err, store.pageRequest)
	}
	if _, err = NewService(store).List(context.Background(), creator.Actor{
		UserID: uuid.New(), Roles: []string{string(creator.RoleOwner)},
	}, PageRequest{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("List(listener) error = %v", err)
	}
}

func moderatorActor() creator.Actor {
	return creator.Actor{UserID: uuid.New(), Roles: []string{creator.GlobalRoleModerator}}
}

type storeStub struct {
	result      Review
	err         error
	review      Review
	calls       int
	page        Page
	pageRequest PageRequest
}

func (stub *storeStub) ListReviews(_ context.Context, page PageRequest) (Page, error) {
	stub.pageRequest = page
	return stub.page, stub.err
}

func (stub *storeStub) RecordReview(_ context.Context, review Review) (Review, error) {
	stub.calls++
	stub.review = review
	if stub.result.ID != uuid.Nil {
		return stub.result, stub.err
	}
	return review, stub.err
}

var _ Store = (*storeStub)(nil)
