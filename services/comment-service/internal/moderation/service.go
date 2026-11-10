package moderation

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
)

const (
	maximumNoteLength = 2000
	defaultPageLimit  = 50
	maximumPageLimit  = 100
)

type Store interface {
	ListModerationCases(context.Context, PageRequest) (Page, error)
	ApplyModerationAction(context.Context, Decision) (Decision, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (service *Service) List(ctx context.Context, actor Actor, page PageRequest) (Page, error) {
	if !authorized(actor) {
		return Page{}, ErrForbidden
	}
	if page.Status != nil && !validStatus(*page.Status) {
		return Page{}, fmt.Errorf("%w: invalid report status", ErrInvalid)
	}
	if page.Limit == 0 {
		page.Limit = defaultPageLimit
	}
	if page.Limit < 1 || page.Limit > maximumPageLimit || page.Offset < 0 {
		return Page{}, fmt.Errorf("%w: invalid pagination", ErrInvalid)
	}
	result, err := service.store.ListModerationCases(ctx, page)
	if err != nil {
		return Page{}, fmt.Errorf("list comment moderation cases: %w", err)
	}
	return result, nil
}

func (service *Service) Apply(ctx context.Context, actor Actor, input Input) (Decision, error) {
	if !authorized(actor) {
		return Decision{}, ErrForbidden
	}
	if input.ReportID == uuid.Nil || !validAction(input.Action) {
		return Decision{}, fmt.Errorf("%w: report id and supported action are required", ErrInvalid)
	}
	input.Note = strings.TrimSpace(input.Note)
	if input.Note == "" || !utf8.ValidString(input.Note) || utf8.RuneCountInString(input.Note) > maximumNoteLength {
		return Decision{}, fmt.Errorf("%w: note must contain from 1 to 2000 characters", ErrInvalid)
	}
	result, err := service.store.ApplyModerationAction(ctx, Decision{
		ID: uuid.New(), ReportID: input.ReportID, ModeratorUserID: actor.UserID,
		Action: input.Action, Note: input.Note,
	})
	if err != nil {
		return Decision{}, fmt.Errorf("apply comment moderation action: %w", err)
	}
	return result, nil
}

func authorized(actor Actor) bool {
	return actor.UserID != uuid.Nil &&
		(slices.Contains(actor.Roles, "moderator") || slices.Contains(actor.Roles, "admin"))
}

func validAction(action Action) bool {
	switch action {
	case ActionStartReview, ActionHideComment, ActionRestoreComment,
		ActionResolveReport, ActionRejectReport:
		return true
	default:
		return false
	}
}

func validStatus(status comment.ReportStatus) bool {
	switch status {
	case comment.ReportStatusPending, comment.ReportStatusReviewing,
		comment.ReportStatusResolved, comment.ReportStatusRejected:
		return true
	default:
		return false
	}
}
