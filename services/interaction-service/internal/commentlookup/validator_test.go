package commentlookup

import (
	"context"
	"errors"
	"testing"
	"time"

	commentv1 "github.com/bevzick/Infinitune/services/interaction-service/api/comment/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidatorAcceptsOnlyExactVisibleComment(t *testing.T) {
	commentID := uuid.New()
	tests := []struct {
		name     string
		response *commentv1.ValidateCommentIdsResponse
		err      error
		want     error
	}{
		{name: "visible", response: &commentv1.ValidateCommentIdsResponse{ValidCommentIds: []string{commentID.String()}}},
		{name: "missing", response: &commentv1.ValidateCommentIdsResponse{MissingCommentIds: []string{commentID.String()}}, want: interaction.ErrCommentNotFound},
		{name: "timeout", err: status.Error(codes.DeadlineExceeded, "timeout"), want: interaction.ErrCommentTimeout},
		{name: "unavailable", err: status.Error(codes.Unavailable, "down"), want: interaction.ErrCommentUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := NewValidator(validationStub{response: test.response, err: test.err})
			validator.timeout = time.Second
			err := validator.ValidateComment(context.Background(), commentID)
			if !errors.Is(err, test.want) {
				t.Fatalf("ValidateComment() error = %v, want %v", err, test.want)
			}
		})
	}
}

type validationStub struct {
	response *commentv1.ValidateCommentIdsResponse
	err      error
}

func (stub validationStub) ValidateCommentIds(
	context.Context, *commentv1.ValidateCommentIdsRequest, ...grpc.CallOption,
) (*commentv1.ValidateCommentIdsResponse, error) {
	return stub.response, stub.err
}
