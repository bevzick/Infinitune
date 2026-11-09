package creator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	Create(context.Context, Account, Member) (Account, error)
	ByID(context.Context, uuid.UUID) (Account, error)
	ListForUser(context.Context, uuid.UUID) ([]Account, error)
	Member(context.Context, uuid.UUID, uuid.UUID) (Member, error)
	Update(context.Context, Account, time.Time) (Account, error)
	TransitionVerification(
		context.Context,
		uuid.UUID,
		VerificationStatus,
		VerificationStatus,
	) (Account, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) List(ctx context.Context, actorID uuid.UUID) ([]Account, error) {
	if actorID == uuid.Nil {
		return nil, ErrForbidden
	}
	accounts, err := service.store.ListForUser(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("list creator accounts: %w", err)
	}
	return accounts, nil
}

func (service *Service) ByID(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
) (Account, error) {
	if actorID == uuid.Nil {
		return Account{}, ErrForbidden
	}
	if accountID == uuid.Nil {
		return Account{}, validationError("creator_id", "must be a UUID")
	}
	account, err := service.store.ByID(ctx, accountID)
	if err != nil {
		return Account{}, fmt.Errorf("read creator account: %w", err)
	}
	membership, err := service.store.Member(ctx, accountID, actorID)
	if errors.Is(err, ErrNotFound) {
		return Account{}, ErrForbidden
	}
	if err != nil {
		return Account{}, fmt.Errorf("read creator membership: %w", err)
	}
	if !HasPermission(membership.Role, PermissionAccountRead) {
		return Account{}, ErrForbidden
	}
	return account, nil
}

func (service *Service) Create(
	ctx context.Context,
	ownerID uuid.UUID,
	input CreateInput,
) (Account, error) {
	account, owner, err := newAccount(ownerID, input)
	if err != nil {
		return Account{}, err
	}
	created, err := service.store.Create(ctx, account, owner)
	if err != nil {
		return Account{}, fmt.Errorf("create creator account: %w", err)
	}
	return created, nil
}

func (service *Service) Update(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
	input UpdateInput,
) (Account, error) {
	current, err := service.owned(ctx, actorID, accountID)
	if err != nil {
		return Account{}, err
	}
	if current.VerificationStatus == VerificationPending {
		return Account{}, ErrInvalidTransition
	}
	changed := false
	if input.Type != nil {
		accountType, validationErr := validAccountType(*input.Type)
		if validationErr != nil {
			return Account{}, validationErr
		}
		if accountType != current.Type {
			current.Type = accountType
			changed = true
		}
	}
	if input.DisplayName != nil {
		displayName, validationErr := validDisplayName(*input.DisplayName)
		if validationErr != nil {
			return Account{}, validationErr
		}
		if displayName != current.DisplayName {
			current.DisplayName = displayName
			changed = true
		}
	}
	if !changed {
		return current, nil
	}
	expectedUpdatedAt := current.UpdatedAt
	if current.VerificationStatus == VerificationVerified {
		current.VerificationStatus = VerificationUnverified
	}
	updated, err := service.store.Update(ctx, current, expectedUpdatedAt)
	if err != nil {
		return Account{}, fmt.Errorf("update creator account: %w", err)
	}
	return updated, nil
}

func (service *Service) SubmitVerification(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
) (Account, error) {
	current, err := service.owned(ctx, actorID, accountID)
	if err != nil {
		return Account{}, err
	}
	if current.VerificationStatus != VerificationUnverified &&
		current.VerificationStatus != VerificationRejected {
		return Account{}, ErrInvalidTransition
	}
	updated, err := service.store.TransitionVerification(
		ctx, current.ID, current.VerificationStatus, VerificationPending,
	)
	if err != nil {
		return Account{}, fmt.Errorf("submit creator verification: %w", err)
	}
	return updated, nil
}

func (service *Service) ReviewVerification(
	ctx context.Context,
	actor Actor,
	accountID uuid.UUID,
	decision VerificationStatus,
) (Account, error) {
	if actor.UserID == uuid.Nil ||
		(!slices.Contains(actor.Roles, GlobalRoleModerator) &&
			!slices.Contains(actor.Roles, GlobalRoleAdmin)) {
		return Account{}, ErrForbidden
	}
	if decision != VerificationVerified && decision != VerificationRejected {
		return Account{}, validationError("decision", "must be verified or rejected")
	}
	if accountID == uuid.Nil {
		return Account{}, validationError("creator_id", "must be a UUID")
	}
	if _, err := service.store.Member(ctx, accountID, actor.UserID); err == nil {
		return Account{}, ErrForbidden
	} else if !errors.Is(err, ErrNotFound) {
		return Account{}, fmt.Errorf("check verification reviewer membership: %w", err)
	}
	current, err := service.store.ByID(ctx, accountID)
	if err != nil {
		return Account{}, fmt.Errorf("read creator account for verification review: %w", err)
	}
	if current.VerificationStatus != VerificationPending {
		return Account{}, ErrInvalidTransition
	}
	updated, err := service.store.TransitionVerification(
		ctx, current.ID, VerificationPending, decision,
	)
	if err != nil {
		return Account{}, fmt.Errorf("review creator verification: %w", err)
	}
	return updated, nil
}

func (service *Service) RequireVerified(ctx context.Context, accountID uuid.UUID) error {
	if accountID == uuid.Nil {
		return validationError("creator_id", "must be a UUID")
	}
	account, err := service.store.ByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("read creator verification: %w", err)
	}
	if account.VerificationStatus != VerificationVerified {
		return ErrVerificationRequired
	}
	return nil
}

func (service *Service) owned(
	ctx context.Context,
	actorID uuid.UUID,
	accountID uuid.UUID,
) (Account, error) {
	if actorID == uuid.Nil {
		return Account{}, ErrForbidden
	}
	if accountID == uuid.Nil {
		return Account{}, validationError("creator_id", "must be a UUID")
	}
	account, err := service.store.ByID(ctx, accountID)
	if err != nil {
		return Account{}, fmt.Errorf("read creator account: %w", err)
	}
	membership, err := service.store.Member(ctx, account.ID, actorID)
	if errors.Is(err, ErrNotFound) {
		return Account{}, ErrForbidden
	}
	if err != nil {
		return Account{}, fmt.Errorf("read creator account membership: %w", err)
	}
	if membership.Role != RoleOwner {
		return Account{}, ErrForbidden
	}
	return account, nil
}
