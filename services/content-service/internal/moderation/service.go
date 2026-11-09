package moderation

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
)

const maximumCommentLength = 2000

type Store interface {
	RecordReview(context.Context, Review) (Review, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service { return &Service{store: store} }

func (service *Service) Review(
	ctx context.Context,
	actor creator.Actor,
	input Input,
) (Review, error) {
	if actor.UserID == uuid.Nil || !isModerator(actor.Roles) {
		return Review{}, ErrForbidden
	}
	validated, err := validateInput(input)
	if err != nil {
		return Review{}, err
	}
	result, err := service.store.RecordReview(ctx, Review{
		ID: uuid.New(), ReleaseID: validated.ReleaseID,
		ReviewerUserID: actor.UserID, Decision: validated.Decision,
		Comment: validated.Comment,
	})
	if err != nil {
		return Review{}, fmt.Errorf("record moderation review: %w", err)
	}
	return result, nil
}

func validateInput(input Input) (Input, error) {
	if input.ReleaseID == uuid.Nil {
		return Input{}, validationError("release_id", "must be a UUID")
	}
	switch input.Decision {
	case DecisionApproved, DecisionRejected, DecisionChangesRequested:
	default:
		return Input{}, validationError(
			"decision", "must be approved, rejected, or changes_requested",
		)
	}
	input.Comment = strings.TrimSpace(input.Comment)
	if !utf8.ValidString(input.Comment) || utf8.RuneCountInString(input.Comment) > maximumCommentLength {
		return Input{}, validationError("comment", "must contain at most 2000 characters")
	}
	if input.Decision != DecisionApproved && input.Comment == "" {
		return Input{}, validationError("comment", "is required for this decision")
	}
	return input, nil
}

func isModerator(roles []string) bool {
	return slices.Contains(roles, creator.GlobalRoleModerator) ||
		slices.Contains(roles, creator.GlobalRoleAdmin)
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
