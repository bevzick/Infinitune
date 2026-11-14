package auth

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/google/uuid"
)

func TestRegistrationServiceCreatesListenerWithNormalizedEmail(t *testing.T) {
	t.Parallel()

	store := &registrationStoreFake{}
	passwords := &passwordManagerFake{hash: "encoded-password"}
	service := NewRegistrationService(store, passwords)

	result, err := service.Register(context.Background(), RegisterInput{
		Email:    "  Listener@Example.COM ",
		Password: "long-enough-password",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.ID == uuid.Nil || result.Email != "listener@example.com" || result.Status != AccountStatusActive {
		t.Fatalf("Register() result = %+v", result)
	}
	if store.params.RoleName != RoleListener || store.params.User.PasswordHash != "encoded-password" {
		t.Fatalf("CreateUserWithRole() params = %+v", store.params)
	}
	if passwords.hashedPlainText != "long-enough-password" {
		t.Fatalf("Hash() input = %q", passwords.hashedPlainText)
	}
}

func TestRegistrationServiceRejectsInvalidInputBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input RegisterInput
		want  error
	}{
		{name: "invalid email", input: RegisterInput{Email: "invalid", Password: "long-enough-password"}, want: ErrInvalidEmail},
		{name: "short password", input: RegisterInput{Email: "user@example.com", Password: "short"}, want: ErrInvalidPassword},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &registrationStoreFake{}
			passwords := &passwordManagerFake{hash: "encoded-password"}
			service := NewRegistrationService(store, passwords)
			if _, err := service.Register(context.Background(), test.input); !errors.Is(err, test.want) {
				t.Fatalf("Register() error = %v, want %v", err, test.want)
			}
			if store.calls != 0 {
				t.Fatalf("CreateUserWithRole() calls = %d", store.calls)
			}
		})
	}
}

func TestLoginServiceIssuesSessionAndRecordsLogin(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	store := &loginStoreFake{
		user: User{
			ID:           userID,
			Email:        "listener@example.com",
			PasswordHash: "encoded-password",
			Status:       AccountStatusActive,
		},
		roles: []string{RoleListener, RoleCreator},
	}
	passwords := &passwordManagerFake{hash: "dummy-hash", verify: true}
	tokens := &tokenIssuerFake{}
	sessions := &refreshManagerFake{}
	service, err := NewLoginService(store, passwords, tokens, sessions)
	if err != nil {
		t.Fatalf("NewLoginService() error = %v", err)
	}

	issued, err := service.Login(context.Background(), LoginInput{
		Email:    "LISTENER@example.com",
		Password: "long-enough-password",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if issued.AccessToken.Value != "access-token" || issued.RefreshToken.Value != "refresh-token" {
		t.Fatalf("Login() = %+v", issued)
	}
	if tokens.userID != userID || !slices.Equal(tokens.roles, store.roles) {
		t.Fatalf("IssueAccessToken() user = %v, roles = %v", tokens.userID, tokens.roles)
	}
	if sessions.createdFor != userID || store.recordedLogin.IsZero() {
		t.Fatalf("session user = %v, recorded login = %v", sessions.createdFor, store.recordedLogin)
	}
}

func TestLoginServiceRejectsInvalidAndUnavailableAccounts(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	tests := []struct {
		name       string
		storeError error
		status     AccountStatus
		verified   bool
		want       error
	}{
		{name: "unknown user", storeError: ErrInvalidCredentials, verified: true, want: ErrInvalidCredentials},
		{name: "wrong password", status: AccountStatusActive, want: ErrInvalidCredentials},
		{name: "blocked", status: AccountStatusBlocked, verified: true, want: ErrAccountUnavailable},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &loginStoreFake{
				user:      User{ID: userID, PasswordHash: "encoded-password", Status: test.status},
				findError: test.storeError,
			}
			passwords := &passwordManagerFake{hash: "dummy-hash", verify: test.verified}
			tokens := &tokenIssuerFake{}
			sessions := &refreshManagerFake{}
			service, err := NewLoginService(store, passwords, tokens, sessions)
			if err != nil {
				t.Fatalf("NewLoginService() error = %v", err)
			}

			_, err = service.Login(context.Background(), LoginInput{
				Email:    "listener@example.com",
				Password: "long-enough-password",
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("Login() error = %v, want %v", err, test.want)
			}
			if sessions.createdFor != uuid.Nil || tokens.userID != uuid.Nil {
				t.Fatal("Login() issued tokens for a rejected account")
			}
		})
	}
}

func TestRefreshServiceUsesCurrentUserRoles(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	store := &loginStoreFake{
		user:  User{ID: userID, Status: AccountStatusActive},
		roles: []string{RoleListener, RoleModerator},
	}
	tokens := &tokenIssuerFake{}
	sessions := &refreshManagerFake{
		rotated: session.IssuedRefreshToken{
			Value:     "new-refresh-token",
			SessionID: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	service := NewRefreshService(store, tokens, sessions)

	issued, err := service.Refresh(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if issued.RefreshToken.Value != "new-refresh-token" || issued.AccessToken.Value != "access-token" {
		t.Fatalf("Refresh() = %+v", issued)
	}
	if !slices.Equal(tokens.roles, []string{RoleListener, RoleModerator}) {
		t.Fatalf("IssueAccessToken() roles = %v", tokens.roles)
	}
}

type registrationStoreFake struct {
	params CreateUserWithRoleParams
	err    error
	calls  int
}

func (s *registrationStoreFake) CreateUserWithRole(
	_ context.Context,
	params CreateUserWithRoleParams,
) error {
	s.calls++
	s.params = params
	return s.err
}

type passwordManagerFake struct {
	hash            string
	hashError       error
	verify          bool
	verifyError     error
	hashedPlainText string
}

func (p *passwordManagerFake) Hash(plainText string) (string, error) {
	p.hashedPlainText = plainText
	return p.hash, p.hashError
}

func (p *passwordManagerFake) Verify(_, _ string) (bool, error) {
	return p.verify, p.verifyError
}

type loginStoreFake struct {
	user          User
	roles         []string
	findError     error
	rolesError    error
	recordError   error
	recordedLogin time.Time
}

func (s *loginStoreFake) FindUserByEmail(_ context.Context, _ string) (User, error) {
	return s.user, s.findError
}

func (s *loginStoreFake) FindUserByID(_ context.Context, _ uuid.UUID) (User, error) {
	return s.user, s.findError
}

func (s *loginStoreFake) ListRoleNamesForUser(_ context.Context, _ uuid.UUID) ([]string, error) {
	return append([]string(nil), s.roles...), s.rolesError
}

func (s *loginStoreFake) RecordLogin(_ context.Context, _ uuid.UUID, loggedInAt time.Time) error {
	s.recordedLogin = loggedInAt
	return s.recordError
}

type tokenIssuerFake struct {
	userID uuid.UUID
	roles  []string
	err    error
}

func (i *tokenIssuerFake) IssueAccessToken(
	userID uuid.UUID,
	roles []string,
	issuedAt time.Time,
) (AccessToken, error) {
	i.userID = userID
	i.roles = append([]string(nil), roles...)
	return AccessToken{
		Value:     "access-token",
		Type:      "Bearer",
		ExpiresAt: issuedAt.Add(time.Minute),
	}, i.err
}

type refreshManagerFake struct {
	createdFor uuid.UUID
	rotated    session.IssuedRefreshToken
	err        error
}

func (m *refreshManagerFake) Create(_ context.Context, userID uuid.UUID) (session.IssuedRefreshToken, error) {
	m.createdFor = userID
	return session.IssuedRefreshToken{Value: "refresh-token", UserID: userID}, m.err
}

func (m *refreshManagerFake) Find(_ context.Context, _ string) (session.RefreshSession, error) {
	return session.RefreshSession{}, m.err
}

func (m *refreshManagerFake) Rotate(_ context.Context, _ string) (session.IssuedRefreshToken, error) {
	return m.rotated, m.err
}

func (m *refreshManagerFake) Revoke(_ context.Context, _ string) (bool, error) {
	return true, m.err
}
