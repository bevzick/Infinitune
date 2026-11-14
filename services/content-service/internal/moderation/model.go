package moderation

import (
	"errors"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

var (
	ErrInvalid    = errors.New("invalid moderation review")
	ErrNotFound   = errors.New("release not found")
	ErrForbidden  = errors.New("moderator role required")
	ErrSelfReview = errors.New("creator member cannot review own release")
	ErrConflict   = errors.New("release submission already reviewed or unavailable")
)

type Decision string

const (
	DecisionApproved         Decision = "approved"
	DecisionRejected         Decision = "rejected"
	DecisionChangesRequested Decision = "changes_requested"
)

type Review struct {
	ID                   uuid.UUID
	ReleaseID            uuid.UUID
	ReviewerUserID       uuid.UUID
	Decision             Decision
	Comment              string
	ReviewedSubmissionAt time.Time
	ReleaseStatus        release.Status
	CreatedAt            time.Time
}

type Input struct {
	ReleaseID uuid.UUID
	Decision  Decision
	Comment   string
}

type PageRequest struct {
	Limit  int
	Offset int
}

type Page struct {
	Items  []Review
	Limit  int
	Offset int
	Total  int64
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string { return err.Field + ": " + err.Reason }
func (err *ValidationError) Unwrap() error { return ErrInvalid }
