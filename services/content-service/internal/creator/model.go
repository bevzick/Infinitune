package creator

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid              = errors.New("invalid creator account")
	ErrNotFound             = errors.New("creator account not found")
	ErrForbidden            = errors.New("creator account operation forbidden")
	ErrConflict             = errors.New("creator account changed concurrently")
	ErrInvalidTransition    = errors.New("invalid verification status transition")
	ErrVerificationRequired = errors.New("verified creator account required")
)

type AccountType string

const (
	TypeArtist      AccountType = "artist"
	TypeLabel       AccountType = "label"
	TypeDistributor AccountType = "distributor"
)

type VerificationStatus string

const (
	VerificationUnverified VerificationStatus = "unverified"
	VerificationPending    VerificationStatus = "pending"
	VerificationVerified   VerificationStatus = "verified"
	VerificationRejected   VerificationStatus = "rejected"
)

type MemberRole string

const (
	RoleOwner    MemberRole = "owner"
	RoleManager  MemberRole = "manager"
	RoleEditor   MemberRole = "editor"
	RoleUploader MemberRole = "uploader"
)

const (
	GlobalRoleAdmin     = "admin"
	GlobalRoleModerator = "moderator"
)

type Account struct {
	ID                 uuid.UUID
	Type               AccountType
	DisplayName        string
	VerificationStatus VerificationStatus
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Member struct {
	CreatorID uuid.UUID
	UserID    uuid.UUID
	Role      MemberRole
	CreatedAt time.Time
}

type Actor struct {
	UserID uuid.UUID
	Roles  []string
}

type CreateInput struct {
	Type        AccountType
	DisplayName string
}

type UpdateInput struct {
	Type        *AccountType
	DisplayName *string
}

type ValidationError struct {
	Field  string
	Reason string
}

func (err *ValidationError) Error() string {
	return err.Field + ": " + err.Reason
}

func (err *ValidationError) Unwrap() error {
	return ErrInvalid
}
