package content

import (
	"context"
	"errors"
	"testing"

	contentv1 "github.com/bevzick/Infinitune/services/media-service/api/content/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClientReadsAndTransitionsUploadSession(t *testing.T) {
	uploadID, userID, trackID := uuid.New(), uuid.New(), uuid.New()
	stub := &grpcClientStub{response: &contentv1.UploadSession{
		Id: uploadID.String(), ReleaseTrackId: trackID.String(),
		RequestedByUserId: userID.String(), ObjectKey: "private/source.flac",
		OriginalName: "source.flac", MimeType: "audio/flac", SizeBytes: 1024,
		Status: "pending",
	}}
	client := NewClient(stub)
	session, err := client.Session(context.Background(), userID, uploadID)
	if err != nil || session.ID != uploadID || session.ReleaseTrackID != trackID ||
		session.Status != StatusPending {
		t.Fatalf("Session() = %+v, %v", session, err)
	}
	stub.response.Status = "uploading"
	session, err = client.Transition(context.Background(), uploadID, StatusUploading)
	if err != nil || session.Status != StatusUploading ||
		stub.transitionRequest.GetTargetStatus() != "uploading" {
		t.Fatalf("Transition() = %+v, %v, request %+v", session, err, stub.transitionRequest)
	}
}

func TestClientMapsForbiddenUpload(t *testing.T) {
	client := NewClient(&grpcClientStub{err: status.Error(codes.PermissionDenied, "forbidden")})
	_, err := client.Session(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Session() error = %v", err)
	}
}

func TestClientRejectsMalformedContentResponse(t *testing.T) {
	client := NewClient(&grpcClientStub{response: &contentv1.UploadSession{
		Id: uuid.NewString(), ReleaseTrackId: "invalid", RequestedByUserId: uuid.NewString(),
		ObjectKey: "private/source.flac", OriginalName: "source.flac",
		MimeType: "audio/flac", SizeBytes: 10, Status: "pending",
	}})
	_, err := client.Session(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Session() error = %v", err)
	}
}

type grpcClientStub struct {
	response          *contentv1.UploadSession
	err               error
	transitionRequest *contentv1.TransitionUploadRequest
}

func (stub *grpcClientStub) GetUploadSession(
	context.Context,
	*contentv1.GetUploadSessionRequest,
	...grpc.CallOption,
) (*contentv1.UploadSession, error) {
	return stub.response, stub.err
}

func (stub *grpcClientStub) TransitionUpload(
	_ context.Context,
	request *contentv1.TransitionUploadRequest,
	_ ...grpc.CallOption,
) (*contentv1.UploadSession, error) {
	stub.transitionRequest = request
	return stub.response, stub.err
}
