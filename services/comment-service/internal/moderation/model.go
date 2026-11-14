package moderation

import (
	"errors"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
)

var (
	ErrInvalid   = errors.New("invalid comment moderation request")
	ErrForbidden = errors.New("moderator role required")
	ErrNotFound  = errors.New("comment report not found")
	ErrConflict  = errors.New("comment moderation action conflicts with current state")
)

type Actor struct {
	UserID uuid.UUID
	Roles  []string
}

type Action string

const (
	ActionStartReview    Action = "start_review"
	ActionHideComment    Action = "hide_comment"
	ActionRestoreComment Action = "restore_comment"
	ActionResolveReport  Action = "resolve_report"
	ActionRejectReport   Action = "reject_report"
)

type Input struct {
	ReportID uuid.UUID
	Action   Action
	Note     string
}

type Decision struct {
	ID                     uuid.UUID
	ReportID               uuid.UUID
	ModeratorUserID        uuid.UUID
	Action                 Action
	Note                   string
	PreviousReportStatus   comment.ReportStatus
	ResultingReportStatus  comment.ReportStatus
	PreviousCommentStatus  comment.Status
	ResultingCommentStatus comment.Status
	CreatedAt              time.Time
}

type Case struct {
	Report  comment.Report
	Comment comment.Comment
}

type PageRequest struct {
	Status *comment.ReportStatus
	Limit  int
	Offset int
}

type Page struct {
	Items  []Case
	Limit  int
	Offset int
	Total  int64
}

type ActionPageRequest struct {
	Limit  int
	Offset int
}

type ActionPage struct {
	Items  []Decision
	Limit  int
	Offset int
	Total  int64
}
