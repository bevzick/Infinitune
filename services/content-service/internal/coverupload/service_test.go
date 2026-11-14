package coverupload

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

type coverStoreStub struct {
	target      ReleaseTarget
	sessions    map[uuid.UUID]Session
	createCalls int
}

func (stub *coverStoreStub) ReleaseTarget(_ context.Context, _ uuid.UUID) (ReleaseTarget, error) {
	if stub.target.ReleaseID == uuid.Nil {
		return ReleaseTarget{}, ErrNotFound
	}
	return stub.target, nil
}
func (stub *coverStoreStub) ByRequest(_ context.Context, releaseID, actorID, key uuid.UUID) (Session, error) {
	for _, session := range stub.sessions {
		if session.ReleaseID == releaseID && session.RequestedByUserID == actorID && session.IdempotencyKey == key {
			return session, nil
		}
	}
	return Session{}, ErrNotFound
}
func (stub *coverStoreStub) Create(_ context.Context, session Session) (Session, bool, error) {
	stub.createCalls++
	if stub.sessions == nil {
		stub.sessions = make(map[uuid.UUID]Session)
	}
	for _, old := range stub.sessions {
		if old.ReleaseID == session.ReleaseID && old.RequestedByUserID == session.RequestedByUserID && old.IdempotencyKey == session.IdempotencyKey {
			return old, false, nil
		}
	}
	stub.sessions[session.ID] = session
	return session, true, nil
}
func (stub *coverStoreStub) ByID(_ context.Context, id uuid.UUID) (Session, error) {
	if session, ok := stub.sessions[id]; ok {
		return session, nil
	}
	return Session{}, ErrNotFound
}
func (stub *coverStoreStub) Complete(_ context.Context, id uuid.UUID) (Session, error) {
	session, ok := stub.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	if session.Status == StatusCompleted {
		return session, nil
	}
	if session.Status != StatusUploading {
		return Session{}, ErrInvalidTransition
	}
	if !editableStatus(stub.target.Status) {
		return Session{}, ErrReleaseLocked
	}
	session.Status = StatusCompleted
	stub.sessions[id] = session
	return session, nil
}
func (stub *coverStoreStub) Transition(_ context.Context, id uuid.UUID, previous, next Status) (Session, error) {
	session, ok := stub.sessions[id]
	if !ok || session.Status != previous {
		return Session{}, ErrConflict
	}
	session.Status = next
	stub.sessions[id] = session
	return session, nil
}

type coverAuthorizerStub struct {
	actorID    uuid.UUID
	creatorID  uuid.UUID
	permission creator.Permission
	err        error
}

func (stub *coverAuthorizerStub) Authorize(_ context.Context, actorID, creatorID uuid.UUID, permission creator.Permission) error {
	stub.actorID, stub.creatorID, stub.permission = actorID, creatorID, permission
	return stub.err
}

func validCoverInput(releaseID uuid.UUID) CreateInput {
	return CreateInput{ReleaseID: releaseID, IdempotencyKey: uuid.New(), OriginalName: "cover.WEBP", MIMEType: "image/webp", SizeBytes: 1024}
}

func TestCreateSessionGeneratesPrivateUniqueKeyAndAuthorizes(t *testing.T) {
	releaseID, creatorID, actorID := uuid.New(), uuid.New(), uuid.New()
	store := &coverStoreStub{target: ReleaseTarget{ReleaseID: releaseID, CreatorID: creatorID, Status: release.StatusDraft}}
	auth := &coverAuthorizerStub{}
	service := NewService(store, auth)
	first, err := service.CreateSession(context.Background(), actorID, validCoverInput(releaseID))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == uuid.Nil || first.ReleaseID != releaseID || first.RequestedByUserID != actorID || first.Status != StatusPending {
		t.Fatalf("unexpected session: %+v", first)
	}
	expectedKey := "creator-content/" + creatorID.String() + "/" + releaseID.String() + "/covers/" + first.ID.String() + "/cover.webp"
	// Keys are derived exclusively from server values and a server-generated UUID.
	if first.ObjectKey != expectedKey || strings.Contains(first.ObjectKey, first.OriginalName) {
		t.Fatalf("object key = %q", first.ObjectKey)
	}
	if auth.permission != creator.PermissionContentUpload || auth.actorID != actorID || auth.creatorID != creatorID {
		t.Fatalf("authorization = %+v", auth)
	}
	second, err := service.CreateSession(context.Background(), actorID, validCoverInput(releaseID))
	if err != nil || second.ID == first.ID || second.ObjectKey == first.ObjectKey {
		t.Fatalf("second = %+v, err = %v", second, err)
	}
}

func TestCreateSessionIdempotencyAndConflict(t *testing.T) {
	releaseID, actorID := uuid.New(), uuid.New()
	store := &coverStoreStub{target: ReleaseTarget{ReleaseID: releaseID, CreatorID: uuid.New(), Status: release.StatusDraft}}
	service := NewService(store, &coverAuthorizerStub{})
	input := validCoverInput(releaseID)
	first, err := service.CreateSession(context.Background(), actorID, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.CreateSession(context.Background(), actorID, input)
	if err != nil || again.ID != first.ID || store.createCalls != 1 {
		t.Fatalf("idempotent = %+v, err = %v", again, err)
	}
	input.SizeBytes++
	_, err = service.CreateSession(context.Background(), actorID, input)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestCreateSessionRejectsUneditableOrUnauthorized(t *testing.T) {
	releaseID, actorID := uuid.New(), uuid.New()
	store := &coverStoreStub{target: ReleaseTarget{ReleaseID: releaseID, CreatorID: uuid.New(), Status: release.StatusPendingReview}}
	auth := &coverAuthorizerStub{}
	_, err := NewService(store, auth).CreateSession(context.Background(), actorID, validCoverInput(releaseID))
	if !errors.Is(err, ErrReleaseLocked) || store.createCalls != 0 {
		t.Fatalf("locked = %v", err)
	}
	store.target.Status = release.StatusDraft
	auth.err = creator.ErrForbidden
	_, err = NewService(store, auth).CreateSession(context.Background(), actorID, validCoverInput(releaseID))
	if !errors.Is(err, creator.ErrForbidden) || store.createCalls != 0 {
		t.Fatalf("forbidden = %v", err)
	}
	auth.err = nil
	for _, editable := range []release.Status{release.StatusRejected, release.StatusChangesRequested} {
		store.target.Status = editable
		if _, err := NewService(store, auth).CreateSession(context.Background(), actorID, validCoverInput(releaseID)); err != nil {
			t.Fatalf("status %s: %v", editable, err)
		}
	}
}

func TestCreateSessionRejectsInvalidCoverMetadata(t *testing.T) {
	releaseID := uuid.New()
	cases := []struct {
		name   string
		change func(*CreateInput)
	}{
		{"missing release", func(in *CreateInput) { in.ReleaseID = uuid.Nil }},
		{"missing idempotency", func(in *CreateInput) { in.IdempotencyKey = uuid.Nil }},
		{"empty name", func(in *CreateInput) { in.OriginalName = "" }},
		{"path traversal", func(in *CreateInput) { in.OriginalName = "../cover.jpg" }},
		{"windows path", func(in *CreateInput) { in.OriginalName = `C:\cover.jpg` }},
		{"long name", func(in *CreateInput) { in.OriginalName = strings.Repeat("x", 256) + ".jpg" }},
		{"unsupported extension", func(in *CreateInput) { in.OriginalName = "cover.svg"; in.MIMEType = "image/svg+xml" }},
		{"mismatched MIME", func(in *CreateInput) { in.OriginalName = "cover.jpg" }},
		{"MIME parameters", func(in *CreateInput) { in.MIMEType = "image/webp; charset=utf-8" }},
		{"empty file", func(in *CreateInput) { in.SizeBytes = 0 }},
		{"too large", func(in *CreateInput) { in.SizeBytes = MaximumCoverSizeBytes + 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &coverStoreStub{target: ReleaseTarget{ReleaseID: releaseID, CreatorID: uuid.New(), Status: release.StatusDraft}}
			input := validCoverInput(releaseID)
			tc.change(&input)
			_, err := NewService(store, &coverAuthorizerStub{}).CreateSession(context.Background(), uuid.New(), input)
			if !errors.Is(err, ErrInvalid) || store.createCalls != 0 {
				t.Fatalf("error = %v; calls = %d", err, store.createCalls)
			}
		})
	}
	for _, format := range []struct{ name, mime string }{
		{"cover.jpg", "image/jpeg"}, {"cover.jpeg", "image/jpeg"}, {"cover.png", "image/png"}, {"cover.webp", "image/webp"},
	} {
		t.Run(format.name, func(t *testing.T) {
			input := validCoverInput(releaseID)
			input.OriginalName, input.MIMEType = format.name, format.mime
			if _, err := validateCreateInput(input); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSessionForUserAndTransitions(t *testing.T) {
	releaseID, actorID := uuid.New(), uuid.New()
	store := &coverStoreStub{target: ReleaseTarget{ReleaseID: releaseID, CreatorID: uuid.New(), Status: release.StatusDraft}}
	service := NewService(store, &coverAuthorizerStub{})
	session, err := service.CreateSession(context.Background(), actorID, validCoverInput(releaseID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SessionForUser(context.Background(), uuid.New(), session.ID); !errors.Is(err, creator.ErrForbidden) {
		t.Fatalf("foreign session: %v", err)
	}
	if _, err = service.SessionForUser(context.Background(), actorID, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Transition(context.Background(), session.ID, StatusCompleted); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("invalid transition = %v", err)
	}
	uploading, err := service.Transition(context.Background(), session.ID, StatusUploading)
	if err != nil || uploading.Status != StatusUploading {
		t.Fatalf("uploading = %+v, %v", uploading, err)
	}
	completed, err := service.Complete(context.Background(), session.ID)
	if err != nil || completed.Status != StatusCompleted {
		t.Fatalf("completed = %+v, %v", completed, err)
	}
	if _, err = service.Transition(context.Background(), session.ID, StatusUploading); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("cannot reopen completed = %v", err)
	}
	store.target.Status = release.StatusPublished
	if _, err = service.SessionForUser(context.Background(), actorID, session.ID); !errors.Is(err, ErrReleaseLocked) {
		t.Fatalf("published session = %v", err)
	}
}

var _ Store = (*coverStoreStub)(nil)
