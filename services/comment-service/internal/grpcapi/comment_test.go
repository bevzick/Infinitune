package grpcapi

import (
	"context"
	"net"
	"testing"

	commentv1 "github.com/bevzick/Infinitune/services/comment-service/api/comment/v1"
	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestValidateCommentIdsSeparatesVisibleAndMissingInRequestOrder(t *testing.T) {
	visibleID, missingID := uuid.New(), uuid.New()
	server := NewCommentServer(lookupStub{visible: map[uuid.UUID]struct{}{visibleID: {}}})
	response, err := server.ValidateCommentIds(context.Background(), &commentv1.ValidateCommentIdsRequest{
		CommentIds: []string{missingID.String(), visibleID.String()},
	})
	if err != nil || len(response.GetValidCommentIds()) != 1 ||
		response.GetValidCommentIds()[0] != visibleID.String() ||
		len(response.GetMissingCommentIds()) != 1 || response.GetMissingCommentIds()[0] != missingID.String() {
		t.Fatalf("ValidateCommentIds() = %+v, %v", response, err)
	}
}

func TestValidateCommentIdsRejectsInvalidBatch(t *testing.T) {
	id := uuid.NewString()
	for _, values := range [][]string{nil, {"invalid"}, {id, id}} {
		_, err := NewCommentServer(lookupStub{}).ValidateCommentIds(
			context.Background(), &commentv1.ValidateCommentIdsRequest{CommentIds: values},
		)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("ValidateCommentIds(%v) code = %s, error = %v", values, status.Code(err), err)
		}
	}
}

func TestCommentGRPCTransport(t *testing.T) {
	visibleID := uuid.New()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	commentv1.RegisterCommentServiceServer(server,
		NewCommentServer(lookupStub{visible: map[uuid.UUID]struct{}{visibleID: {}}}))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	connection, err := grpc.NewClient("passthrough:///comment",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	response, err := commentv1.NewCommentServiceClient(connection).ValidateCommentIds(
		context.Background(), &commentv1.ValidateCommentIdsRequest{CommentIds: []string{visibleID.String()}},
	)
	if err != nil || len(response.GetValidCommentIds()) != 1 || response.GetValidCommentIds()[0] != visibleID.String() {
		t.Fatalf("ValidateCommentIds() = %+v, %v", response, err)
	}
}

type lookupStub struct{ visible map[uuid.UUID]struct{} }

func (stub lookupStub) ValidateIDs(_ context.Context, ids []uuid.UUID) (comment.IDValidation, error) {
	result := comment.IDValidation{}
	for _, id := range ids {
		if _, ok := stub.visible[id]; ok {
			result.ValidIDs = append(result.ValidIDs, id)
		} else {
			result.MissingIDs = append(result.MissingIDs, id)
		}
	}
	return result, nil
}
