package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestContentApprovalHandlerProjectsApproval(t *testing.T) {
	store := &approvalStoreStub{}
	eventID, releaseID, creatorID, reviewerID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	payload := []byte(`{"id":"` + eventID.String() + `","type":"content.release.approved.v1",` +
		`"source":"content-service","occurred_at":"2026-11-12T12:00:00Z","data":{` +
		`"release_id":"` + releaseID.String() + `","creator_id":"` + creatorID.String() + `",` +
		`"reviewer_user_id":"` + reviewerID.String() + `","reviewed_submission_at":"2026-11-12T11:00:00Z"}}`)
	if err := NewContentApprovalHandler(store).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if store.approval.EventID != eventID || store.approval.ReleaseID != releaseID ||
		store.approval.CreatorID != creatorID || store.approval.ReviewerUserID != reviewerID ||
		!store.approval.ApprovedAt.Equal(time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("approval = %+v", store.approval)
	}
}

func TestContentApprovalHandlerRejectsMalformedEvent(t *testing.T) {
	payload := []byte(`{"id":"` + uuid.NewString() + `","type":"content.release.approved.v1",` +
		`"source":"content-service","occurred_at":"2026-11-12T12:00:00Z","data":{}}`)
	err := NewContentApprovalHandler(&approvalStoreStub{}).Handle(context.Background(), payload)
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Handle() error = %v, want %v", err, ErrInvalidEvent)
	}
}

type approvalStoreStub struct{ approval ContentReleaseApproval }

func (stub *approvalStoreStub) UpsertContentApproval(_ context.Context, approval ContentReleaseApproval) error {
	stub.approval = approval
	return nil
}
