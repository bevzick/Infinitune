package auth

import (
	"time"

	"github.com/google/uuid"
)

type AccountStatus string

const (
	AccountStatusActive  AccountStatus = "active"
	AccountStatusBlocked AccountStatus = "blocked"
	AccountStatusDeleted AccountStatus = "deleted"
)

type User struct {
	ID            uuid.UUID
	Email         string
	PasswordHash  string
	Status        AccountStatus
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastLoginAt   *time.Time
}

type AdminUser struct {
	User  User
	Roles []string
}

type UserPageRequest struct {
	Status *AccountStatus
	Limit  int
	Offset int
}

type UserPage struct {
	Items  []AdminUser
	Limit  int
	Offset int
	Total  int64
}

type Role struct {
	ID          int16
	Name        string
	Description *string
}

type Identity struct {
	UserID    uuid.UUID
	Roles     []string
	TokenID   uuid.UUID
	IssuedAt  time.Time
	ExpiresAt time.Time
}

const (
	RoleListener  = "listener"
	RoleAdmin     = "admin"
	RoleModerator = "moderator"
	RoleCreator   = "creator"
)
