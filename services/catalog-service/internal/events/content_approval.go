package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const ContentReleaseApprovedV1 = "content.release.approved.v1"

var ErrInvalidEvent = errors.New("invalid event")

type contentApprovalEnvelope struct {
	ID         uuid.UUID              `json:"id"`
	Type       string                 `json:"type"`
	Source     string                 `json:"source"`
	OccurredAt time.Time              `json:"occurred_at"`
	Data       ContentReleaseApproval `json:"data"`
}

type ContentReleaseApproval struct {
	ReleaseID            uuid.UUID `json:"release_id"`
	CreatorID            uuid.UUID `json:"creator_id"`
	ReviewerUserID       uuid.UUID `json:"reviewer_user_id"`
	ReviewedSubmissionAt time.Time `json:"reviewed_submission_at"`
	EventID              uuid.UUID `json:"-"`
	ApprovedAt           time.Time `json:"-"`
}

type ApprovalStore interface {
	UpsertContentApproval(context.Context, ContentReleaseApproval) error
}

type ContentApprovalHandler struct{ store ApprovalStore }

func NewContentApprovalHandler(store ApprovalStore) *ContentApprovalHandler {
	return &ContentApprovalHandler{store: store}
}

func (handler *ContentApprovalHandler) Handle(ctx context.Context, payload []byte) error {
	var event contentApprovalEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("%w: decode content approval event: %v", ErrInvalidEvent, err)
	}
	if event.ID == uuid.Nil || event.Type != ContentReleaseApprovedV1 ||
		event.Source != "content-service" || event.OccurredAt.IsZero() ||
		event.Data.ReleaseID == uuid.Nil || event.Data.CreatorID == uuid.Nil ||
		event.Data.ReviewerUserID == uuid.Nil || event.Data.ReviewedSubmissionAt.IsZero() {
		return fmt.Errorf("%w: incomplete content approval event", ErrInvalidEvent)
	}
	event.Data.EventID = event.ID
	event.Data.ApprovedAt = event.OccurredAt.UTC()
	if err := handler.store.UpsertContentApproval(ctx, event.Data); err != nil {
		return fmt.Errorf("project content approval: %w", err)
	}
	return nil
}
