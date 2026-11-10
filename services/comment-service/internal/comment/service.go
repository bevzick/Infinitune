package comment

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maximumContentLength           = 2000
	maximumReportDescriptionLength = 2000
	defaultPageLimit               = 50
	maximumPageLimit               = 100
)

type Store interface {
	Create(context.Context, Comment) (Comment, error)
	List(context.Context, uuid.UUID, PageRequest) (Page, error)
	UpdateOwned(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error)
	DeleteOwned(context.Context, uuid.UUID, uuid.UUID) error
	CreateReport(context.Context, Report) (Report, error)
	ExistingVisibleIDs(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

type TrackValidator interface {
	ValidateTrack(context.Context, uuid.UUID) error
}

type Service struct {
	store     Store
	validator TrackValidator
}

func NewService(store Store, validator TrackValidator) *Service {
	return &Service{store: store, validator: validator}
}

func (service *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
	input CreateInput,
) (Comment, error) {
	if userID == uuid.Nil {
		return Comment{}, invalidArgument("user id is required")
	}
	if input.TrackID == uuid.Nil {
		return Comment{}, invalidArgument("track id is required")
	}
	if input.ParentID != nil && *input.ParentID == uuid.Nil {
		return Comment{}, invalidArgument("parent id must be a UUID")
	}
	content, err := validateContent(input.Content)
	if err != nil {
		return Comment{}, err
	}
	if err = service.validator.ValidateTrack(ctx, input.TrackID); err != nil {
		return Comment{}, fmt.Errorf("validate track: %w", err)
	}
	created, err := service.store.Create(ctx, Comment{
		ID: uuid.New(), UserID: userID, TrackID: input.TrackID,
		ParentID: input.ParentID, Content: content,
	})
	if err != nil {
		return Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return created, nil
}

func (service *Service) List(
	ctx context.Context,
	trackID uuid.UUID,
	page PageRequest,
) (Page, error) {
	if trackID == uuid.Nil {
		return Page{}, invalidArgument("track id is required")
	}
	page, err := validatePage(page)
	if err != nil {
		return Page{}, err
	}
	if err = service.validator.ValidateTrack(ctx, trackID); err != nil {
		return Page{}, fmt.Errorf("validate track: %w", err)
	}
	result, err := service.store.List(ctx, trackID, page)
	if err != nil {
		return Page{}, fmt.Errorf("list comments: %w", err)
	}
	return result, nil
}

func (service *Service) Update(
	ctx context.Context,
	userID uuid.UUID,
	commentID uuid.UUID,
	content string,
) (Comment, error) {
	if userID == uuid.Nil || commentID == uuid.Nil {
		return Comment{}, invalidArgument("user id and comment id are required")
	}
	content, err := validateContent(content)
	if err != nil {
		return Comment{}, err
	}
	updated, err := service.store.UpdateOwned(ctx, commentID, userID, content)
	if err != nil {
		return Comment{}, fmt.Errorf("update comment: %w", err)
	}
	return updated, nil
}

func (service *Service) Delete(
	ctx context.Context,
	userID uuid.UUID,
	commentID uuid.UUID,
) error {
	if userID == uuid.Nil || commentID == uuid.Nil {
		return invalidArgument("user id and comment id are required")
	}
	if err := service.store.DeleteOwned(ctx, commentID, userID); err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}

func (service *Service) Report(
	ctx context.Context,
	userID uuid.UUID,
	commentID uuid.UUID,
	input ReportInput,
) (Report, error) {
	if userID == uuid.Nil || commentID == uuid.Nil {
		return Report{}, invalidArgument("user id and comment id are required")
	}
	if !validReportReason(input.Reason) {
		return Report{}, invalidArgument("report reason is invalid")
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return Report{}, invalidArgument("report description must not be blank")
	}
	if !utf8.ValidString(description) || utf8.RuneCountInString(description) > maximumReportDescriptionLength {
		return Report{}, invalidArgument("report description must contain at most 2000 characters")
	}
	created, err := service.store.CreateReport(ctx, Report{
		ID: uuid.New(), CommentID: commentID, ReporterUserID: userID,
		Reason: input.Reason, Description: description,
	})
	if err != nil {
		return Report{}, fmt.Errorf("report comment: %w", err)
	}
	return created, nil
}

func (service *Service) ValidateIDs(ctx context.Context, ids []uuid.UUID) (IDValidation, error) {
	existing, err := service.store.ExistingVisibleIDs(ctx, ids)
	if err != nil {
		return IDValidation{}, fmt.Errorf("validate visible comment ids: %w", err)
	}
	result := IDValidation{
		ValidIDs: make([]uuid.UUID, 0, len(ids)), MissingIDs: make([]uuid.UUID, 0),
	}
	for _, id := range ids {
		if _, ok := existing[id]; ok {
			result.ValidIDs = append(result.ValidIDs, id)
		} else {
			result.MissingIDs = append(result.MissingIDs, id)
		}
	}
	return result, nil
}

func validReportReason(reason ReportReason) bool {
	switch reason {
	case ReportReasonSpam, ReportReasonHarassment, ReportReasonHateSpeech,
		ReportReasonSexualContent, ReportReasonViolence, ReportReasonMisinformation,
		ReportReasonOther:
		return true
	default:
		return false
	}
}

func validateContent(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", invalidArgument("content must not be blank")
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximumContentLength {
		return "", invalidArgument("content must contain at most 2000 characters")
	}
	return value, nil
}

func validatePage(page PageRequest) (PageRequest, error) {
	if page.Limit == 0 {
		page.Limit = defaultPageLimit
	}
	if page.Limit < 1 || page.Limit > maximumPageLimit {
		return PageRequest{}, invalidArgument("page limit must be from 1 to 100")
	}
	if page.Offset < 0 {
		return PageRequest{}, invalidArgument("page offset must not be negative")
	}
	return page, nil
}
