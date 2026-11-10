package comment

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidArgument    = errors.New("invalid comment argument")
	ErrNotFound           = errors.New("comment not found")
	ErrForbidden          = errors.New("comment access forbidden")
	ErrNotEditable        = errors.New("comment is not editable")
	ErrParentInvalid      = errors.New("comment parent is invalid")
	ErrRateLimited        = errors.New("comment rate limit exceeded")
	ErrReportDuplicate    = errors.New("comment was already reported by this user")
	ErrReportRateLimited  = errors.New("comment report rate limit exceeded")
	ErrOwnCommentReport   = errors.New("own comment cannot be reported")
	ErrTrackNotFound      = errors.New("catalog track not found")
	ErrCatalogUnavailable = errors.New("catalog service unavailable")
	ErrCatalogTimeout     = errors.New("catalog request timed out")
)

type Status string

const (
	StatusPublished Status = "published"
	StatusHidden    Status = "hidden"
	StatusDeleted   Status = "deleted"
)

type Comment struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TrackID   uuid.UUID
	ParentID  *uuid.UUID
	Content   string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "pending"
	ReportStatusReviewing ReportStatus = "reviewing"
	ReportStatusResolved  ReportStatus = "resolved"
	ReportStatusRejected  ReportStatus = "rejected"
)

type ReportReason string

const (
	ReportReasonSpam           ReportReason = "spam"
	ReportReasonHarassment     ReportReason = "harassment"
	ReportReasonHateSpeech     ReportReason = "hate_speech"
	ReportReasonSexualContent  ReportReason = "sexual_content"
	ReportReasonViolence       ReportReason = "violence"
	ReportReasonMisinformation ReportReason = "misinformation"
	ReportReasonOther          ReportReason = "other"
)

type Report struct {
	ID             uuid.UUID
	CommentID      uuid.UUID
	ReporterUserID uuid.UUID
	Reason         ReportReason
	Description    string
	Status         ReportStatus
	CreatedAt      time.Time
	UpdatedAt      *time.Time
}

type ReportInput struct {
	Reason      ReportReason
	Description string
}

type CreateInput struct {
	TrackID  uuid.UUID
	ParentID *uuid.UUID
	Content  string
}

type PageRequest struct {
	Limit  int
	Offset int
}

type Page struct {
	Items  []Comment
	Limit  int
	Offset int
	Total  int64
}

type IDValidation struct {
	ValidIDs   []uuid.UUID
	MissingIDs []uuid.UUID
}

func invalidArgument(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}
