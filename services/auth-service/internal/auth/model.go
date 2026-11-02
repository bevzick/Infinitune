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

type Role struct {
	ID          int16
	Name        string
	Description *string
}
