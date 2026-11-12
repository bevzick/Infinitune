package creator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceCreatesUnverifiedAccountWithOwner(t *testing.T) {
	ownerID := uuid.New()
	store := &storeStub{}
	service := NewService(store)

	created, err := service.Create(context.Background(), ownerID, CreateInput{
		Type: TypeArtist, DisplayName: "  First Light  ",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == uuid.Nil || created.Type != TypeArtist ||
		created.DisplayName != "First Light" ||
		created.VerificationStatus != VerificationUnverified {
		t.Fatalf("Create() = %+v", created)
	}
	if store.createdOwner.CreatorID != created.ID || store.createdOwner.UserID != ownerID ||
		store.createdOwner.Role != RoleOwner {
		t.Fatalf("created owner = %+v", store.createdOwner)
	}
}

func TestServiceListsAndReadsMemberAccounts(t *testing.T) {
	userID, accountID := uuid.New(), uuid.New()
	account := Account{ID: accountID, DisplayName: "Member Artist"}
	store := &storeStub{
		account: account, accounts: []Account{account},
		member: Member{CreatorID: accountID, UserID: userID, Role: RoleEditor},
	}
	service := NewService(store)
	accounts, err := service.List(context.Background(), userID)
	if err != nil || len(accounts) != 1 || accounts[0].ID != accountID {
		t.Fatalf("List() = %+v, %v", accounts, err)
	}
	loaded, err := service.ByID(context.Background(), userID, accountID)
	if err != nil || loaded.ID != accountID {
		t.Fatalf("ByID() = %+v, %v", loaded, err)
	}
}

func TestServiceHidesAccountFromNonMember(t *testing.T) {
	_, err := NewService(&storeStub{account: Account{ID: uuid.New()}}).
		ByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ByID() error = %v, want %v", err, ErrForbidden)
	}
}

func TestServiceRejectsInvalidCreatorAccount(t *testing.T) {
	tests := []struct {
		name    string
		ownerID uuid.UUID
		input   CreateInput
	}{
		{name: "missing owner", input: CreateInput{Type: TypeArtist, DisplayName: "Artist"}},
		{name: "unsupported type", ownerID: uuid.New(),
			input: CreateInput{Type: "listener", DisplayName: "Listener"}},
		{name: "blank display name", ownerID: uuid.New(),
			input: CreateInput{Type: TypeLabel, DisplayName: "   "}},
		{name: "long display name", ownerID: uuid.New(),
			input: CreateInput{Type: TypeDistributor, DisplayName: strings.Repeat("a", 151)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{}
			_, err := NewService(store).Create(context.Background(), test.ownerID, test.input)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Create() error = %v, want %v", err, ErrInvalid)
			}
			if store.createCalls != 0 {
				t.Fatalf("Create() store calls = %d", store.createCalls)
			}
		})
	}
}

func TestServiceUpdatesOwnedAccountAndResetsVerifiedStatus(t *testing.T) {
	ownerID := uuid.New()
	accountID := uuid.New()
	updatedAt := time.Now().UTC().Add(-time.Hour)
	store := &storeStub{
		account: Account{
			ID: accountID, Type: TypeArtist, DisplayName: "Old Name",
			VerificationStatus: VerificationVerified, UpdatedAt: updatedAt,
		},
		member: Member{CreatorID: accountID, UserID: ownerID, Role: RoleOwner},
	}
	accountType := TypeLabel
	displayName := "  New Name  "

	updated, err := NewService(store).Update(
		context.Background(), ownerID, accountID,
		UpdateInput{Type: &accountType, DisplayName: &displayName},
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Type != TypeLabel || updated.DisplayName != "New Name" ||
		updated.VerificationStatus != VerificationUnverified {
		t.Fatalf("Update() = %+v", updated)
	}
	if !store.expectedUpdatedAt.Equal(updatedAt) {
		t.Fatalf("Update() expected timestamp = %v, want %v", store.expectedUpdatedAt, updatedAt)
	}
}

func TestServiceRestrictsAccountUpdates(t *testing.T) {
	ownerID := uuid.New()
	accountID := uuid.New()
	name := "Changed"
	tests := []struct {
		name      string
		actorID   uuid.UUID
		status    VerificationStatus
		member    Member
		wantError error
	}{
		{name: "non member", actorID: uuid.New(), status: VerificationUnverified,
			wantError: ErrForbidden},
		{name: "non owner member", actorID: uuid.New(), status: VerificationUnverified,
			member: Member{CreatorID: accountID, Role: RoleManager}, wantError: ErrForbidden},
		{name: "pending review", actorID: ownerID, status: VerificationPending,
			member:    Member{CreatorID: accountID, UserID: ownerID, Role: RoleOwner},
			wantError: ErrInvalidTransition},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{account: Account{
				ID: accountID, Type: TypeArtist, DisplayName: "Name",
				VerificationStatus: test.status,
			}, member: test.member}
			if test.member.UserID == uuid.Nil && test.member.Role != "" {
				store.member.UserID = test.actorID
			}
			_, err := NewService(store).Update(
				context.Background(), test.actorID, accountID, UpdateInput{DisplayName: &name},
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Update() error = %v, want %v", err, test.wantError)
			}
			if store.updateCalls != 0 {
				t.Fatalf("Update() store calls = %d", store.updateCalls)
			}
		})
	}
}

func TestServiceSubmitsUnverifiedAndRejectedAccounts(t *testing.T) {
	for _, status := range []VerificationStatus{VerificationUnverified, VerificationRejected} {
		t.Run(string(status), func(t *testing.T) {
			ownerID := uuid.New()
			accountID := uuid.New()
			store := &storeStub{
				account: Account{ID: accountID, VerificationStatus: status},
				member:  Member{CreatorID: accountID, UserID: ownerID, Role: RoleOwner},
			}
			updated, err := NewService(store).SubmitVerification(
				context.Background(), ownerID, accountID,
			)
			if err != nil {
				t.Fatalf("SubmitVerification() error = %v", err)
			}
			if updated.VerificationStatus != VerificationPending ||
				store.transitionExpected != status ||
				store.transitionTarget != VerificationPending {
				t.Fatalf("SubmitVerification() = %+v, transition %s -> %s",
					updated, store.transitionExpected, store.transitionTarget)
			}
		})
	}
}

func TestServiceRejectsInvalidVerificationSubmission(t *testing.T) {
	for _, status := range []VerificationStatus{VerificationPending, VerificationVerified} {
		t.Run(string(status), func(t *testing.T) {
			ownerID := uuid.New()
			accountID := uuid.New()
			store := &storeStub{
				account: Account{ID: accountID, VerificationStatus: status},
				member:  Member{CreatorID: accountID, UserID: ownerID, Role: RoleOwner},
			}
			_, err := NewService(store).SubmitVerification(context.Background(), ownerID, accountID)
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("SubmitVerification() error = %v, want %v", err, ErrInvalidTransition)
			}
		})
	}
}

func TestServiceReviewsPendingVerification(t *testing.T) {
	for _, role := range []string{GlobalRoleModerator, GlobalRoleAdmin} {
		t.Run(role, func(t *testing.T) {
			accountID := uuid.New()
			store := &storeStub{account: Account{
				ID: accountID, VerificationStatus: VerificationPending,
			}}
			updated, err := NewService(store).ReviewVerification(
				context.Background(), Actor{UserID: uuid.New(), Roles: []string{role}},
				accountID, VerificationVerified,
			)
			if err != nil {
				t.Fatalf("ReviewVerification() error = %v", err)
			}
			if updated.VerificationStatus != VerificationVerified ||
				store.transitionExpected != VerificationPending {
				t.Fatalf("ReviewVerification() = %+v", updated)
			}
		})
	}
}

func TestServiceRestrictsVerificationReview(t *testing.T) {
	accountID := uuid.New()
	memberReviewerID := uuid.New()
	tests := []struct {
		name      string
		actor     Actor
		status    VerificationStatus
		decision  VerificationStatus
		member    Member
		wantError error
	}{
		{name: "listener", actor: Actor{UserID: uuid.New(), Roles: []string{"listener"}},
			status: VerificationPending, decision: VerificationVerified, wantError: ErrForbidden},
		{name: "invalid decision", actor: Actor{UserID: uuid.New(), Roles: []string{GlobalRoleModerator}},
			status: VerificationPending, decision: VerificationPending, wantError: ErrInvalid},
		{name: "not pending", actor: Actor{UserID: uuid.New(), Roles: []string{GlobalRoleAdmin}},
			status: VerificationUnverified, decision: VerificationRejected,
			wantError: ErrInvalidTransition},
		{name: "account member cannot review", actor: Actor{
			UserID: memberReviewerID, Roles: []string{GlobalRoleModerator},
		}, status: VerificationPending, decision: VerificationVerified,
			member:    Member{CreatorID: accountID, UserID: memberReviewerID, Role: RoleOwner},
			wantError: ErrForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &storeStub{
				account: Account{ID: accountID, VerificationStatus: test.status},
				member:  test.member,
			}
			_, err := NewService(store).ReviewVerification(
				context.Background(), test.actor, accountID, test.decision,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("ReviewVerification() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestServiceRequiresVerifiedAccountForProtectedActions(t *testing.T) {
	accountID := uuid.New()
	for _, status := range []VerificationStatus{
		VerificationUnverified, VerificationPending, VerificationRejected, VerificationVerified,
	} {
		t.Run(string(status), func(t *testing.T) {
			err := NewService(&storeStub{account: Account{
				ID: accountID, VerificationStatus: status,
			}}).RequireVerified(context.Background(), accountID)
			if status == VerificationVerified && err != nil {
				t.Fatalf("RequireVerified() error = %v", err)
			}
			if status != VerificationVerified && !errors.Is(err, ErrVerificationRequired) {
				t.Fatalf("RequireVerified() error = %v, want %v", err, ErrVerificationRequired)
			}
		})
	}
}

func TestServiceListsVerificationQueueForModerator(t *testing.T) {
	moderatorID := uuid.New()
	store := &storeStub{verificationPage: VerificationPage{
		Items: []Account{{ID: uuid.New(), VerificationStatus: VerificationPending}},
		Total: 1,
	}}
	page, err := NewService(store).ListVerification(context.Background(), Actor{
		UserID: moderatorID, Roles: []string{GlobalRoleModerator},
	}, VerificationPageRequest{Limit: 25, Offset: 5})
	if err != nil || page.Total != 1 || store.reviewerID != moderatorID ||
		store.verificationRequest.Limit != 25 || store.verificationRequest.Offset != 5 ||
		store.verificationRequest.Status == nil ||
		*store.verificationRequest.Status != VerificationPending {
		t.Fatalf("ListVerification() = %+v, %v; store = %+v", page, err, store)
	}
}

func TestServiceRejectsUnauthorizedOrInvalidVerificationQueue(t *testing.T) {
	service := NewService(&storeStub{})
	if _, err := service.ListVerification(context.Background(), Actor{
		UserID: uuid.New(), Roles: []string{"listener"},
	}, VerificationPageRequest{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListVerification(listener) error = %v", err)
	}
	unverified := VerificationUnverified
	if _, err := service.ListVerification(context.Background(), Actor{
		UserID: uuid.New(), Roles: []string{GlobalRoleAdmin},
	}, VerificationPageRequest{Status: &unverified}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ListVerification(unverified) error = %v", err)
	}
}

type storeStub struct {
	account             Account
	accounts            []Account
	member              Member
	createdOwner        Member
	expectedUpdatedAt   time.Time
	transitionExpected  VerificationStatus
	transitionTarget    VerificationStatus
	verificationPage    VerificationPage
	verificationRequest VerificationPageRequest
	reviewerID          uuid.UUID
	createCalls         int
	updateCalls         int
}

func (store *storeStub) ListVerification(
	_ context.Context,
	reviewerID uuid.UUID,
	request VerificationPageRequest,
) (VerificationPage, error) {
	store.reviewerID, store.verificationRequest = reviewerID, request
	return store.verificationPage, nil
}

func (store *storeStub) ListForUser(context.Context, uuid.UUID) ([]Account, error) {
	return store.accounts, nil
}

func (store *storeStub) Create(
	_ context.Context,
	account Account,
	owner Member,
) (Account, error) {
	store.createCalls++
	store.account = account
	store.createdOwner = owner
	return account, nil
}

func (store *storeStub) ByID(context.Context, uuid.UUID) (Account, error) {
	return store.account, nil
}

func (store *storeStub) Member(
	_ context.Context,
	creatorID uuid.UUID,
	userID uuid.UUID,
) (Member, error) {
	if store.member.CreatorID != creatorID || store.member.UserID != userID {
		return Member{}, ErrNotFound
	}
	return store.member, nil
}

func (store *storeStub) Update(
	_ context.Context,
	account Account,
	expectedUpdatedAt time.Time,
) (Account, error) {
	store.updateCalls++
	store.account = account
	store.expectedUpdatedAt = expectedUpdatedAt
	return account, nil
}

func (store *storeStub) TransitionVerification(
	_ context.Context,
	_ uuid.UUID,
	expected VerificationStatus,
	target VerificationStatus,
) (Account, error) {
	store.transitionExpected = expected
	store.transitionTarget = target
	store.account.VerificationStatus = target
	return store.account, nil
}

var _ Store = (*storeStub)(nil)
