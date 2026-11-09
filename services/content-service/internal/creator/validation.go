package creator

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maximumDisplayNameLength = 150

func newAccount(ownerID uuid.UUID, input CreateInput) (Account, Member, error) {
	if ownerID == uuid.Nil {
		return Account{}, Member{}, validationError("owner_id", "must be a UUID")
	}
	accountType, err := validAccountType(input.Type)
	if err != nil {
		return Account{}, Member{}, err
	}
	displayName, err := validDisplayName(input.DisplayName)
	if err != nil {
		return Account{}, Member{}, err
	}
	accountID := uuid.New()
	return Account{
		ID: accountID, Type: accountType, DisplayName: displayName,
		VerificationStatus: VerificationUnverified,
	}, Member{CreatorID: accountID, UserID: ownerID, Role: RoleOwner}, nil
}

func validAccountType(value AccountType) (AccountType, error) {
	switch value {
	case TypeArtist, TypeLabel, TypeDistributor:
		return value, nil
	default:
		return "", validationError("account_type", "must be artist, label, or distributor")
	}
}

func validDisplayName(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", validationError("display_name", "must not be blank")
	}
	if !utf8.ValidString(trimmed) || utf8.RuneCountInString(trimmed) > maximumDisplayNameLength {
		return "", validationError("display_name", "must contain at most 150 characters")
	}
	return trimmed, nil
}

func validationError(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}
