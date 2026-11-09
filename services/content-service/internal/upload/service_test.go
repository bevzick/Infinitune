package upload

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func TestServiceCreatesPendingUploadSession(t *testing.T) {
	actorID, creatorID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &storeStub{target: TrackTarget{
		TrackID: trackID, ReleaseID: releaseID, CreatorID: creatorID,
		ReleaseStatus: release.StatusDraft,
	}}
	authorizer := &authorizerStub{}
	session, err := NewService(store, authorizer).CreateSession(
		context.Background(), actorID, CreateInput{
			ReleaseTrackID: trackID, IdempotencyKey: uuid.New(),
			OriginalName: "Master.FLAC", MIMEType: "audio/flac", SizeBytes: 64 * 1024 * 1024,
		},
	)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.ID == uuid.Nil || session.ReleaseTrackID != trackID ||
		session.RequestedByUserID != actorID || session.Status != StatusPending ||
		session.OriginalName != "Master.FLAC" || session.MIMEType != "audio/flac" {
		t.Fatalf("CreateSession() = %+v", session)
	}
	wantPrefix := "creator-content/" + creatorID.String() + "/" + releaseID.String() +
		"/" + trackID.String() + "/" + session.ID.String() + "/source.flac"
	if session.ObjectKey != wantPrefix || strings.Contains(session.ObjectKey, session.OriginalName) {
		t.Fatalf("ObjectKey = %q", session.ObjectKey)
	}
	if authorizer.permission != creator.PermissionContentUpload ||
		authorizer.actorID != actorID || authorizer.creatorID != creatorID {
		t.Fatalf("authorization = %+v", authorizer)
	}
}

func TestServiceAcceptsSupportedAudioFormats(t *testing.T) {
	tests := []struct {
		name, fileName, mimeType string
	}{
		{name: "wav", fileName: "source.wav", mimeType: "audio/wav"},
		{name: "legacy wav", fileName: "source.wav", mimeType: "audio/x-wav"},
		{name: "flac", fileName: "source.flac", mimeType: "audio/flac"},
		{name: "mp3", fileName: "source.mp3", mimeType: "audio/mpeg"},
		{name: "m4a", fileName: "source.m4a", mimeType: "audio/mp4"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trackID := uuid.New()
			_, err := NewService(&storeStub{target: TrackTarget{
				TrackID: trackID, ReleaseID: uuid.New(), CreatorID: uuid.New(),
				ReleaseStatus: release.StatusDraft,
			}}, &authorizerStub{}).CreateSession(context.Background(), uuid.New(), CreateInput{
				ReleaseTrackID: trackID, IdempotencyKey: uuid.New(),
				OriginalName: test.fileName, MIMEType: test.mimeType, SizeBytes: 1,
			})
			if err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}
		})
	}
}

func TestServiceRejectsInvalidUploadBeforeLookup(t *testing.T) {
	trackID := uuid.New()
	tests := []struct {
		name  string
		input CreateInput
	}{
		{name: "missing track", input: CreateInput{IdempotencyKey: uuid.New(), OriginalName: "a.wav", MIMEType: "audio/wav", SizeBytes: 1}},
		{name: "missing idempotency key", input: CreateInput{ReleaseTrackID: trackID, OriginalName: "a.wav", MIMEType: "audio/wav", SizeBytes: 1}},
		{name: "path traversal", input: CreateInput{ReleaseTrackID: trackID, IdempotencyKey: uuid.New(), OriginalName: "../a.wav", MIMEType: "audio/wav", SizeBytes: 1}},
		{name: "unsupported extension", input: CreateInput{ReleaseTrackID: trackID, IdempotencyKey: uuid.New(), OriginalName: "a.exe", MIMEType: "audio/wav", SizeBytes: 1}},
		{name: "mismatched mime", input: CreateInput{ReleaseTrackID: trackID, IdempotencyKey: uuid.New(), OriginalName: "a.mp3", MIMEType: "audio/wav", SizeBytes: 1}},
		{name: "mime parameters", input: CreateInput{ReleaseTrackID: trackID, IdempotencyKey: uuid.New(), OriginalName: "a.mp3", MIMEType: "audio/mpeg; charset=utf-8", SizeBytes: 1}},
		{name: "empty file", input: CreateInput{ReleaseTrackID: trackID, IdempotencyKey: uuid.New(), OriginalName: "a.wav", MIMEType: "audio/wav", SizeBytes: 0}},
		{name: "oversized file", input: CreateInput{ReleaseTrackID: trackID, IdempotencyKey: uuid.New(), OriginalName: "a.wav", MIMEType: "audio/wav", SizeBytes: MaximumAudioSizeBytes + 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{}
			_, err := NewService(store, &authorizerStub{}).CreateSession(
				context.Background(), uuid.New(), test.input,
			)
			if !errors.Is(err, ErrInvalid) || store.targetCalls != 0 {
				t.Fatalf("CreateSession() error = %v, target calls = %d", err, store.targetCalls)
			}
		})
	}
}

func TestServiceRejectsForeignCreatorAndLockedRelease(t *testing.T) {
	trackID := uuid.New()
	input := CreateInput{
		ReleaseTrackID: trackID, IdempotencyKey: uuid.New(),
		OriginalName: "track.wav", MIMEType: "audio/wav", SizeBytes: 10,
	}
	tests := []struct {
		name      string
		status    release.Status
		authError error
		wantError error
	}{
		{name: "foreign creator", status: release.StatusDraft, authError: creator.ErrForbidden, wantError: creator.ErrForbidden},
		{name: "pending review", status: release.StatusPendingReview, wantError: ErrReleaseLocked},
		{name: "published", status: release.StatusPublished, wantError: ErrReleaseLocked},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{target: TrackTarget{
				TrackID: trackID, ReleaseID: uuid.New(), CreatorID: uuid.New(),
				ReleaseStatus: test.status,
			}}
			_, err := NewService(store, &authorizerStub{err: test.authError}).CreateSession(
				context.Background(), uuid.New(), input,
			)
			if !errors.Is(err, test.wantError) || store.createCalls != 0 {
				t.Fatalf("CreateSession() error = %v, create calls = %d", err, store.createCalls)
			}
		})
	}
}

func TestServiceReturnsIdempotentSession(t *testing.T) {
	actorID, trackID, idempotencyKey := uuid.New(), uuid.New(), uuid.New()
	existing := Session{
		ID: uuid.New(), ReleaseTrackID: trackID, RequestedByUserID: actorID,
		IdempotencyKey: idempotencyKey, ObjectKey: "existing/source.wav",
		OriginalName: "source.wav", MIMEType: "audio/wav", SizeBytes: 100,
		Status: StatusPending,
	}
	store := &storeStub{target: TrackTarget{
		TrackID: trackID, ReleaseID: uuid.New(), CreatorID: uuid.New(),
		ReleaseStatus: release.StatusDraft,
	}, createResult: existing, created: false}
	result, err := NewService(store, &authorizerStub{}).CreateSession(
		context.Background(), actorID, CreateInput{
			ReleaseTrackID: trackID, IdempotencyKey: idempotencyKey,
			OriginalName: "source.wav", MIMEType: "audio/wav", SizeBytes: 100,
		},
	)
	if err != nil || result.ID != existing.ID || result.ObjectKey != existing.ObjectKey {
		t.Fatalf("CreateSession() = %+v, %v", result, err)
	}

	store.createResult.SizeBytes = 101
	_, err = NewService(store, &authorizerStub{}).CreateSession(
		context.Background(), actorID, CreateInput{
			ReleaseTrackID: trackID, IdempotencyKey: idempotencyKey,
			OriginalName: "source.wav", MIMEType: "audio/wav", SizeBytes: 100,
		},
	)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("CreateSession(conflict) error = %v", err)
	}
}

func TestServiceControlsUploadStatusTransitions(t *testing.T) {
	uploadID := uuid.New()
	tests := []struct {
		name    string
		current Status
		target  Status
		allowed bool
	}{
		{name: "start", current: StatusPending, target: StatusUploading, allowed: true},
		{name: "fail pending", current: StatusPending, target: StatusFailed, allowed: true},
		{name: "complete", current: StatusUploading, target: StatusCompleted, allowed: true},
		{name: "fail uploading", current: StatusUploading, target: StatusFailed, allowed: true},
		{name: "retry", current: StatusFailed, target: StatusPending, allowed: true},
		{name: "skip upload", current: StatusPending, target: StatusCompleted},
		{name: "completed terminal", current: StatusCompleted, target: StatusPending},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{byIDResult: Session{ID: uploadID, Status: test.current}}
			result, err := NewService(store, &authorizerStub{}).Transition(
				context.Background(), uploadID, test.target,
			)
			if test.allowed {
				if err != nil || result.Status != test.target || store.transitionCalls != 1 {
					t.Fatalf("Transition() = %+v, %v, calls %d", result, err, store.transitionCalls)
				}
			} else if !errors.Is(err, ErrInvalidTransition) || store.transitionCalls != 0 {
				t.Fatalf("Transition() error = %v, calls %d", err, store.transitionCalls)
			}
		})
	}
}

func TestServiceReturnsUploadOnlyToRequester(t *testing.T) {
	actorID, uploadID := uuid.New(), uuid.New()
	service := NewService(&storeStub{byIDResult: Session{
		ID: uploadID, RequestedByUserID: actorID,
	}}, &authorizerStub{})
	session, err := service.SessionForUser(context.Background(), actorID, uploadID)
	if err != nil || session.ID != uploadID {
		t.Fatalf("SessionForUser() = %+v, %v", session, err)
	}
	if _, err = service.SessionForUser(
		context.Background(), uuid.New(), uploadID,
	); !errors.Is(err, creator.ErrForbidden) {
		t.Fatalf("SessionForUser(foreign) error = %v", err)
	}
}

type storeStub struct {
	target          TrackTarget
	targetCalls     int
	requestResult   Session
	requestError    error
	createResult    Session
	created         bool
	createCalls     int
	byIDResult      Session
	transitionCalls int
}

func (store *storeStub) TrackTarget(context.Context, uuid.UUID) (TrackTarget, error) {
	store.targetCalls++
	return store.target, nil
}

func (store *storeStub) ByRequest(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID,
) (Session, error) {
	if store.requestError != nil {
		return Session{}, store.requestError
	}
	if store.requestResult.ID != uuid.Nil {
		return store.requestResult, nil
	}
	return Session{}, ErrNotFound
}

func (store *storeStub) Create(_ context.Context, session Session) (Session, bool, error) {
	store.createCalls++
	if store.createResult.ID != uuid.Nil || !store.created {
		if store.createResult.ID != uuid.Nil {
			return store.createResult, store.created, nil
		}
	}
	return session, true, nil
}

func (store *storeStub) ByID(context.Context, uuid.UUID) (Session, error) {
	return store.byIDResult, nil
}

func (store *storeStub) Transition(
	_ context.Context,
	_ uuid.UUID,
	_ Status,
	target Status,
) (Session, error) {
	store.transitionCalls++
	store.byIDResult.Status = target
	return store.byIDResult, nil
}

type authorizerStub struct {
	actorID    uuid.UUID
	creatorID  uuid.UUID
	permission creator.Permission
	err        error
}

func (stub *authorizerStub) Authorize(
	_ context.Context,
	actorID uuid.UUID,
	creatorID uuid.UUID,
	permission creator.Permission,
) error {
	stub.actorID = actorID
	stub.creatorID = creatorID
	stub.permission = permission
	return stub.err
}

var _ Store = (*storeStub)(nil)
