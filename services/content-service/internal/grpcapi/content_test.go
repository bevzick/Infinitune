package grpcapi

import (
	"context"
	"testing"

	contentv1 "github.com/bevzick/Infinitune/services/content-service/api/content/v1"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestContentServerReturnsAccessDecision(t *testing.T) {
	checker := &checkerStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleOwner,
		VerificationStatus: creator.VerificationVerified,
	}}
	response, err := NewContentServer(checker, nil, nil).CheckPermission(context.Background(),
		&contentv1.CheckPermissionRequest{
			CreatorId: uuid.NewString(), UserId: uuid.NewString(),
			Permission: string(creator.PermissionContentPublish),
		})
	if err != nil || !response.GetAllowed() || response.GetMemberRole() != "owner" {
		t.Fatalf("CheckPermission() = %+v, %v", response, err)
	}
}

func TestContentServerRejectsInvalidIdentifiers(t *testing.T) {
	_, err := NewContentServer(&checkerStub{}, nil, nil).CheckPermission(context.Background(),
		&contentv1.CheckPermissionRequest{CreatorId: "bad", UserId: uuid.NewString()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CheckPermission() code = %v", status.Code(err))
	}
}

func TestContentServerReturnsUploadSessionForUser(t *testing.T) {
	uploadID, userID, trackID := uuid.New(), uuid.New(), uuid.New()
	sessions := &uploadSessionsStub{session: upload.Session{
		ID: uploadID, ReleaseTrackID: trackID, RequestedByUserID: userID,
		ObjectKey: "private/source.flac", OriginalName: "source.flac",
		MIMEType: "audio/flac", SizeBytes: 1024, Status: upload.StatusPending,
	}}
	response, err := NewContentServer(&checkerStub{}, sessions, nil).GetUploadSession(
		context.Background(), &contentv1.GetUploadSessionRequest{
			UploadId: uploadID.String(), UserId: userID.String(),
		},
	)
	if err != nil || response.GetId() != uploadID.String() ||
		response.GetReleaseTrackId() != trackID.String() || response.GetStatus() != "pending" {
		t.Fatalf("GetUploadSession() = %+v, %v", response, err)
	}
}

func TestContentServerMapsUploadTransitionConflict(t *testing.T) {
	_, err := NewContentServer(&checkerStub{}, &uploadSessionsStub{
		err: upload.ErrConflict,
	}, nil).TransitionUpload(context.Background(), &contentv1.TransitionUploadRequest{
		UploadId: uuid.NewString(), TargetStatus: "completed",
	})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("TransitionUpload() code = %v", status.Code(err))
	}
}

func TestContentServerReturnsPlaybackDecision(t *testing.T) {
	trackID := uuid.New()
	access := &playbackAccessStub{allowed: true}
	response, err := NewContentServer(&checkerStub{}, nil, access).CheckTrackPlayback(
		context.Background(), &contentv1.CheckTrackPlaybackRequest{
			ReleaseTrackId: trackID.String(),
		},
	)
	if err != nil || !response.GetAllowed() || access.trackID != trackID {
		t.Fatalf("CheckTrackPlayback() = %+v, %v; access = %+v", response, err, access)
	}
}

func TestContentServerRejectsInvalidPlaybackTrack(t *testing.T) {
	_, err := NewContentServer(&checkerStub{}, nil, &playbackAccessStub{}).CheckTrackPlayback(
		context.Background(), &contentv1.CheckTrackPlaybackRequest{ReleaseTrackId: "bad"},
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CheckTrackPlayback() code = %v", status.Code(err))
	}
}

type checkerStub struct {
	decision creator.AccessDecision
	err      error
}

type uploadSessionsStub struct {
	session upload.Session
	err     error
}

type playbackAccessStub struct {
	allowed bool
	err     error
	trackID uuid.UUID
}

func (stub *playbackAccessStub) IsTrackPlayable(
	_ context.Context,
	trackID uuid.UUID,
) (bool, error) {
	stub.trackID = trackID
	return stub.allowed, stub.err
}

func (stub *uploadSessionsStub) SessionForUser(
	context.Context, uuid.UUID, uuid.UUID,
) (upload.Session, error) {
	return stub.session, stub.err
}

func (stub *uploadSessionsStub) Transition(
	context.Context, uuid.UUID, upload.Status,
) (upload.Session, error) {
	return stub.session, stub.err
}

func (stub *checkerStub) Check(
	context.Context, uuid.UUID, uuid.UUID, creator.Permission,
) (creator.AccessDecision, error) {
	return stub.decision, stub.err
}
