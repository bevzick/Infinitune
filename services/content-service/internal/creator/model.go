package creator

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid                  = errors.New("invalid creator account")
	ErrNotFound                 = errors.New("creator account not found")
	ErrForbidden                = errors.New("creator account operation forbidden")
	ErrConflict                 = errors.New("creator account changed concurrently")
	ErrMemberExists             = errors.New("creator member already exists")
	ErrOwnerProtected           = errors.New("creator account owner is protected")
	ErrTargetUserNotFound       = errors.New("creator member user not found")
	ErrUserDirectoryUnavailable = errors.New("user directory unavailable")
	ErrInvalidTransition        = errors.New("invalid verification status transition")
	ErrVerificationRequired     = errors.New("verified creator account required")
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

type Permission string

const (
	PermissionAccountRead        Permission = "creator.account.read"
	PermissionAccountUpdate      Permission = "creator.account.update"
	PermissionVerificationSubmit Permission = "creator.verification.submit"
	PermissionMembersRead        Permission = "creator.members.read"
	PermissionMembersManage      Permission = "creator.members.manage"
	PermissionContentEdit        Permission = "creator.content.edit"
	PermissionContentUpload      Permission = "creator.content.upload"
	PermissionContentPublish     Permission = "creator.content.publish"
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

type VerificationPageRequest struct {
	Status *VerificationStatus
	Limit  int
	Offset int
}

type VerificationPage struct {
	Items  []Account
	Limit  int
	Offset int
	Total  int64
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

type AccessDecision struct {
	Allowed            bool
	Role               MemberRole
	VerificationStatus VerificationStatus
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
