package catalog

import (
	"context"
	"errors"
	"testing"

	catalogv1 "github.com/bevzick/Infinitune/services/comment-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTrackValidatorMapsCatalogResponses(t *testing.T) {
	trackID := uuid.New()
	tests := []struct {
		name   string
		client *validationStub
		want   error
	}{
		{name: "available", client: &validationStub{trackID: trackID}},
		{name: "missing", client: &validationStub{}, want: comment.ErrTrackNotFound},
		{name: "timeout", client: &validationStub{err: status.Error(codes.DeadlineExceeded, "timeout")}, want: comment.ErrCatalogTimeout},
		{name: "unavailable", client: &validationStub{err: status.Error(codes.Unavailable, "offline")}, want: comment.ErrCatalogUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := NewTrackValidator(test.client).ValidateTrack(context.Background(), trackID)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

type validationStub struct {
	trackID uuid.UUID
	err     error
}

func (stub *validationStub) ValidateTrackIds(context.Context, *catalogv1.ValidateTrackIdsRequest, ...grpc.CallOption) (*catalogv1.ValidateTrackIdsResponse, error) {
	if stub.err != nil {
		return nil, stub.err
	}
	if stub.trackID == uuid.Nil {
		return &catalogv1.ValidateTrackIdsResponse{MissingTrackIds: []string{uuid.NewString()}}, nil
	}
	return &catalogv1.ValidateTrackIdsResponse{ValidTrackIds: []string{stub.trackID.String()}}, nil
}
