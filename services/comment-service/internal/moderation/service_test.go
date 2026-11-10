package moderation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
)

func TestServiceAllowsModeratorAndAdmin(t *testing.T) {
	for _, role := range []string{"moderator", "admin"} {
		t.Run(role, func(t *testing.T) {
			store := &moderationStoreStub{}
			service := NewService(store)
			actor := Actor{UserID: uuid.New(), Roles: []string{"listener", role}}
			result, err := service.Apply(context.Background(), actor, Input{
				ReportID: uuid.New(), Action: ActionHideComment, Note: "  Violates policy  ",
			})
			if err != nil || result.ModeratorUserID != actor.UserID || result.Note != "Violates policy" {
				t.Fatalf("Apply() = %+v, %v", result, err)
			}
		})
	}
}

func TestServiceRejectsUnauthorizedOrInvalidModeration(t *testing.T) {
	moderator := Actor{UserID: uuid.New(), Roles: []string{"moderator"}}
	tests := []struct {
		name  string
		actor Actor
		input Input
		want  error
	}{
		{name: "listener", actor: Actor{UserID: uuid.New(), Roles: []string{"listener"}}, input: validInput(), want: ErrForbidden},
		{name: "missing actor", actor: Actor{Roles: []string{"admin"}}, input: validInput(), want: ErrForbidden},
		{name: "missing report", actor: moderator, input: Input{Action: ActionResolveReport, Note: "Done"}, want: ErrInvalid},
		{name: "invalid action", actor: moderator, input: Input{ReportID: uuid.New(), Action: "delete", Note: "No"}, want: ErrInvalid},
		{name: "blank note", actor: moderator, input: Input{ReportID: uuid.New(), Action: ActionRejectReport}, want: ErrInvalid},
		{name: "long note", actor: moderator, input: Input{ReportID: uuid.New(), Action: ActionRejectReport, Note: strings.Repeat("x", 2001)}, want: ErrInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &moderationStoreStub{}
			_, err := NewService(store).Apply(context.Background(), test.actor, test.input)
			if !errors.Is(err, test.want) || store.applyCalls != 0 {
				t.Fatalf("Apply() error/calls = %v/%d, want %v/0", err, store.applyCalls, test.want)
			}
		})
	}
}

func TestServiceListsValidatedModerationQueue(t *testing.T) {
	status := comment.ReportStatusPending
	store := &moderationStoreStub{}
	service := NewService(store)
	page, err := service.List(context.Background(), Actor{UserID: uuid.New(), Roles: []string{"moderator"}}, PageRequest{Status: &status})
	if err != nil || page.Limit != 50 || store.page.Limit != 50 || store.page.Status == nil {
		t.Fatalf("List() = %+v, %v; store page = %+v", page, err, store.page)
	}
	invalid := comment.ReportStatus("unknown")
	if _, err = service.List(context.Background(), Actor{UserID: uuid.New(), Roles: []string{"admin"}}, PageRequest{Status: &invalid}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("List(invalid status) error = %v", err)
	}
}

func validInput() Input {
	return Input{ReportID: uuid.New(), Action: ActionStartReview, Note: "Reviewing"}
}

type moderationStoreStub struct {
	page       PageRequest
	applyCalls int
}

func (stub *moderationStoreStub) ListModerationCases(_ context.Context, page PageRequest) (Page, error) {
	stub.page = page
	return Page{Items: []Case{}, Limit: page.Limit, Offset: page.Offset}, nil
}

func (stub *moderationStoreStub) ApplyModerationAction(_ context.Context, value Decision) (Decision, error) {
	stub.applyCalls++
	value.CreatedAt = time.Now()
	return value, nil
}
