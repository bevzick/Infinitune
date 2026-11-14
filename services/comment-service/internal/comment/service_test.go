package comment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceCreatesCommentForVerifiedOwnerAndValidatedTrack(t *testing.T) {
	userID, trackID := uuid.New(), uuid.New()
	store := &storeStub{}
	validator := &trackValidatorStub{}
	service := NewService(store, validator)
	created, err := service.Create(context.Background(), userID, CreateInput{
		TrackID: trackID, Content: "  Useful comment  ",
	})
	if err != nil || created.UserID != userID || created.TrackID != trackID ||
		created.Content != "Useful comment" || created.ID == uuid.Nil {
		t.Fatalf("Create() = %+v, %v", created, err)
	}
	if validator.trackID != trackID || store.created.UserID != userID {
		t.Fatalf("validated track %s, stored owner %s", validator.trackID, store.created.UserID)
	}
}

func TestServiceRejectsInvalidOrMissingTrackBeforeStorage(t *testing.T) {
	tests := []struct {
		name, content string
		validatorErr  error
	}{
		{name: "blank", content: "   "},
		{name: "too long", content: strings.Repeat("x", 2001)},
		{name: "missing track", content: "valid", validatorErr: ErrTrackNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{}
			service := NewService(store, &trackValidatorStub{err: test.validatorErr})
			_, err := service.Create(context.Background(), uuid.New(), CreateInput{
				TrackID: uuid.New(), Content: test.content,
			})
			if err == nil || store.createCalls != 0 {
				t.Fatalf("error = %v, store calls = %d", err, store.createCalls)
			}
		})
	}
}

func TestServiceUsesVerifiedOwnerForUpdateAndDelete(t *testing.T) {
	userID, commentID := uuid.New(), uuid.New()
	store := &storeStub{}
	service := NewService(store, &trackValidatorStub{})
	if _, err := service.Update(context.Background(), userID, commentID, "Updated"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if store.userID != userID || store.commentID != commentID {
		t.Fatalf("update owner/id = %s/%s", store.userID, store.commentID)
	}
	if err := service.Delete(context.Background(), userID, commentID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.userID != userID || store.commentID != commentID {
		t.Fatalf("delete owner/id = %s/%s", store.userID, store.commentID)
	}
}

func TestServiceListValidatesTrackAndPagination(t *testing.T) {
	trackID := uuid.New()
	store := &storeStub{}
	validator := &trackValidatorStub{}
	service := NewService(store, validator)
	page, err := service.List(context.Background(), trackID, PageRequest{})
	if err != nil || page.Limit != 50 || store.page != (PageRequest{Limit: 50}) || validator.trackID != trackID {
		t.Fatalf("List() = %+v, %v; store page = %+v", page, err, store.page)
	}
	if _, err = service.List(context.Background(), trackID, PageRequest{Limit: 101}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("List(invalid) error = %v", err)
	}
}

func TestServiceReportsCommentForVerifiedReporter(t *testing.T) {
	reporterID, commentID := uuid.New(), uuid.New()
	store := &storeStub{}
	service := NewService(store, &trackValidatorStub{})
	created, err := service.Report(context.Background(), reporterID, commentID, ReportInput{
		Reason: ReportReasonHarassment, Description: "  Personal attack  ",
	})
	if err != nil || created.ReporterUserID != reporterID || created.CommentID != commentID ||
		created.Description != "Personal attack" || created.Status != ReportStatusPending {
		t.Fatalf("Report() = %+v, %v", created, err)
	}
}

func TestServiceRejectsInvalidReportBeforeStorage(t *testing.T) {
	tests := []ReportInput{
		{Reason: "invented", Description: "Details"},
		{Reason: ReportReasonSpam, Description: "  "},
		{Reason: ReportReasonOther, Description: strings.Repeat("x", 2001)},
	}
	for _, input := range tests {
		store := &storeStub{}
		service := NewService(store, &trackValidatorStub{})
		if _, err := service.Report(context.Background(), uuid.New(), uuid.New(), input); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("Report(%+v) error = %v", input, err)
		}
		if store.reportCalls != 0 {
			t.Fatalf("Report(%+v) store calls = %d", input, store.reportCalls)
		}
	}
}

type trackValidatorStub struct {
	trackID uuid.UUID
	err     error
}

func (stub *trackValidatorStub) ValidateTrack(_ context.Context, id uuid.UUID) error {
	stub.trackID = id
	return stub.err
}

type storeStub struct {
	created     Comment
	userID      uuid.UUID
	commentID   uuid.UUID
	page        PageRequest
	createCalls int
	reportCalls int
}

func (stub *storeStub) Create(_ context.Context, value Comment) (Comment, error) {
	stub.created = value
	stub.createCalls++
	value.Status = StatusPublished
	value.CreatedAt = time.Now()
	value.UpdatedAt = value.CreatedAt
	return value, nil
}
func (stub *storeStub) List(_ context.Context, _ uuid.UUID, page PageRequest) (Page, error) {
	stub.page = page
	return Page{Items: []Comment{}, Limit: page.Limit, Offset: page.Offset}, nil
}
func (stub *storeStub) UpdateOwned(_ context.Context, id, userID uuid.UUID, content string) (Comment, error) {
	stub.commentID, stub.userID = id, userID
	return Comment{ID: id, UserID: userID, Content: content}, nil
}
func (stub *storeStub) DeleteOwned(_ context.Context, id, userID uuid.UUID) error {
	stub.commentID, stub.userID = id, userID
	return nil
}
func (stub *storeStub) CreateReport(_ context.Context, value Report) (Report, error) {
	stub.reportCalls++
	value.Status = ReportStatusPending
	value.CreatedAt = time.Now()
	return value, nil
}
func (stub *storeStub) ExistingVisibleIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, nil
}
