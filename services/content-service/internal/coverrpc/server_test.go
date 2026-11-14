package coverrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"

	"github.com/bevzick/Infinitune/services/content-service/internal/coverupload"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type fakeSessions struct {
	session coverupload.Session
	err     error
}

func (f fakeSessions) SessionForUser(_ context.Context, actor, upload uuid.UUID) (coverupload.Session, error) {
	if f.err != nil {
		return coverupload.Session{}, f.err
	}
	if actor != f.session.RequestedByUserID || upload != f.session.ID {
		return coverupload.Session{}, creator.ErrForbidden
	}
	return f.session, nil
}
func (f fakeSessions) Transition(_ context.Context, id uuid.UUID, next coverupload.Status) (coverupload.Session, error) {
	if f.err != nil {
		return coverupload.Session{}, f.err
	}
	s := f.session
	s.Status = next
	return s, nil
}
func (f fakeSessions) Complete(_ context.Context, id uuid.UUID) (coverupload.Session, error) {
	if f.err != nil {
		return coverupload.Session{}, f.err
	}
	s := f.session
	s.Status = coverupload.StatusCompleted
	return s, nil
}
func TestCoverRPCResponsesAndPermissions(t *testing.T) {
	s := coverupload.Session{ID: uuid.New(), ReleaseID: uuid.New(), RequestedByUserID: uuid.New(), MIMEType: "image/png", ObjectKey: "private/cover.png", SizeBytes: 345, Status: coverupload.StatusPending}
	server := &Server{sessions: fakeSessions{session: s}}
	req, _ := structpb.NewStruct(map[string]any{"upload_id": s.ID.String(), "user_id": s.RequestedByUserID.String()})
	result, err := server.GetSession(context.Background(), req)
	if err != nil || result.Fields["object_key"].GetStringValue() != s.ObjectKey || result.Fields["size_bytes"].GetNumberValue() != 345 {
		t.Fatalf("result=%v err=%v", result, err)
	}
	foreign, _ := structpb.NewStruct(map[string]any{"upload_id": s.ID.String(), "user_id": uuid.New().String()})
	_, err = server.GetSession(context.Background(), foreign)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("foreign=%v", err)
	}
	_, err = server.GetSession(context.Background(), nil)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing=%v", err)
	}
	completed, err := server.Complete(context.Background(), req)
	if err != nil || completed.Fields["status"].GetStringValue() != "completed" {
		t.Fatalf("complete=%v err=%v", completed, err)
	}
}
func TestCoverRPCMapsDomainFailures(t *testing.T) {
	s := coverupload.Session{ID: uuid.New()}
	server := &Server{sessions: fakeSessions{session: s, err: coverupload.ErrReleaseLocked}}
	req, _ := structpb.NewStruct(map[string]any{"upload_id": s.ID.String()})
	_, err := server.Complete(context.Background(), req)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("err=%v", err)
	}
}

func TestCoverRPCDispatchThroughGRPC(t *testing.T) {
	session := coverupload.Session{
		ID: uuid.New(), ReleaseID: uuid.New(), RequestedByUserID: uuid.New(),
		ObjectKey: "private/image.png", MIMEType: "image/png", SizeBytes: 4096,
		Status: coverupload.StatusPending,
	}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	Register(server, fakeSessions{session: session})
	defer server.Stop()
	go func() { _ = server.Serve(listener) }()
	connection, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	req, _ := structpb.NewStruct(map[string]any{"upload_id": session.ID.String(), "user_id": session.RequestedByUserID.String()})
	var resp structpb.Struct
	if err := connection.Invoke(context.Background(), GetSessionMethod, req, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.GetFields()["id"].GetStringValue() != session.ID.String() {
		t.Fatalf("bad response: %v", resp.String())
	}
}
