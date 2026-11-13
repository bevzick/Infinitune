package adminbootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/google/uuid"
)

func TestPromoteAssignsAdminToAnActiveRegisteredUser(t *testing.T) {
	userID := uuid.New()
	store := &storeStub{user: auth.User{
		ID: userID, Email: "admin@example.com", Status: auth.AccountStatusActive,
	}}
	result, err := Promote(context.Background(), store, "  ADMIN@example.com ")
	if err != nil || result.UserID != userID || result.Email != "admin@example.com" ||
		result.AlreadyAssigned || store.email != "admin@example.com" || store.assignedID != userID {
		t.Fatalf("Promote() = %+v, %v; store = %+v", result, err, store)
	}
}

func TestPromoteIsIdempotent(t *testing.T) {
	store := &storeStub{
		user:      auth.User{ID: uuid.New(), Email: "admin@example.com", Status: auth.AccountStatusActive},
		assignErr: auth.ErrRoleAlreadyAssigned,
	}
	result, err := Promote(context.Background(), store, "admin@example.com")
	if err != nil || !result.AlreadyAssigned {
		t.Fatalf("Promote() = %+v, %v", result, err)
	}
}

func TestPromoteRejectsInvalidEmailAndInactiveUser(t *testing.T) {
	store := &storeStub{}
	if _, err := Promote(context.Background(), store, "not-an-email"); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("Promote(invalid) error = %v", err)
	}
	store.user = auth.User{ID: uuid.New(), Email: "blocked@example.com", Status: auth.AccountStatusBlocked}
	if _, err := Promote(context.Background(), store, "blocked@example.com"); !errors.Is(err, ErrUserInactive) {
		t.Fatalf("Promote(blocked) error = %v", err)
	}
	if store.assignedID != uuid.Nil {
		t.Fatalf("inactive user was assigned admin: %s", store.assignedID)
	}
}

type storeStub struct {
	user       auth.User
	findErr    error
	assignErr  error
	email      string
	assignedID uuid.UUID
}

func (store *storeStub) FindUserByEmail(_ context.Context, email string) (auth.User, error) {
	store.email = email
	return store.user, store.findErr
}

func (store *storeStub) AssignAdminRole(_ context.Context, id uuid.UUID, _ time.Time) error {
	store.assignedID = id
	return store.assignErr
}
