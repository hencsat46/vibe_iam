package usecase

import (
	"context"
	"fmt"

	"temp/internal/domain"

	"github.com/google/uuid"
)

type RoleUseCase struct {
	roleRepo         domain.RoleRepository
	resourceRepo     domain.ResourceRepository
	accessObjectRepo domain.AccessObjectRepository
}

func NewRoleUseCase(rr domain.RoleRepository, resr domain.ResourceRepository, aor domain.AccessObjectRepository) *RoleUseCase {
	return &RoleUseCase{roleRepo: rr, resourceRepo: resr, accessObjectRepo: aor}
}

func (uc *RoleUseCase) Add(ctx context.Context, accessObjectUID string, resourceUIDs []string, name, displayName, description string, permissions []string, attrs map[string]string, labels domain.Labels) (*domain.Role, error) {
	ao, err := uc.accessObjectRepo.GetByUID(ctx, accessObjectUID)
	if err != nil {
		return nil, fmt.Errorf("get access object: %w", err)
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		return nil, domain.ErrAccessObjectDraft
	}

	if err := uc.validateResourcesBelongToAO(ctx, accessObjectUID, resourceUIDs); err != nil {
		return nil, err
	}

	r := &domain.Role{
		UID:             uuid.New().String(),
		AccessObjectUID: accessObjectUID,
		ResourceUIDs:    resourceUIDs,
		Name:            name,
		DisplayName:     displayName,
		Description:     description,
		Permissions:     permissions,
		Attributes:      attrs,
		Labels:          labels,
	}

	if err := uc.roleRepo.Add(ctx, r); err != nil {
		return nil, fmt.Errorf("add role: %w", err)
	}
	return r, nil
}

func (uc *RoleUseCase) Get(ctx context.Context, uid string) (*domain.Role, error) {
	r, err := uc.roleRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get role: %w", err)
	}
	return r, nil
}

func (uc *RoleUseCase) List(ctx context.Context, f domain.RoleFilter) ([]domain.Role, int32, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	return uc.roleRepo.List(ctx, f)
}

func (uc *RoleUseCase) Update(ctx context.Context, uid string, upd domain.RoleUpdate) (*domain.Role, error) {
	existing, err := uc.roleRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get role: %w", err)
	}

	ao, err := uc.accessObjectRepo.GetByUID(ctx, existing.AccessObjectUID)
	if err != nil {
		return nil, fmt.Errorf("get access object: %w", err)
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		return nil, domain.ErrAccessObjectDraft
	}

	if err := uc.validateResourcesBelongToAO(ctx, existing.AccessObjectUID, upd.ResourceUIDs); err != nil {
		return nil, err
	}

	r, err := uc.roleRepo.Update(ctx, uid, upd)
	if err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}
	return r, nil
}

func (uc *RoleUseCase) Remove(ctx context.Context, uid string) error {
	existing, err := uc.roleRepo.GetByUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("get role: %w", err)
	}

	ao, err := uc.accessObjectRepo.GetByUID(ctx, existing.AccessObjectUID)
	if err != nil {
		return fmt.Errorf("get access object: %w", err)
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		return domain.ErrAccessObjectDraft
	}

	if err := uc.roleRepo.Remove(ctx, uid); err != nil {
		return fmt.Errorf("remove role: %w", err)
	}
	return nil
}

func (uc *RoleUseCase) AddChildRole(ctx context.Context, parentRoleUID string, resourceUIDs []string, name, displayName, description string, permissions []string, attrs map[string]string, labels domain.Labels) (*domain.Role, error) {
	parent, err := uc.roleRepo.GetByUID(ctx, parentRoleUID)
	if err != nil {
		return nil, fmt.Errorf("get parent role: %w", err)
	}

	ao, err := uc.accessObjectRepo.GetByUID(ctx, parent.AccessObjectUID)
	if err != nil {
		return nil, fmt.Errorf("get access object: %w", err)
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		return nil, domain.ErrAccessObjectDraft
	}

	if err := uc.validateResourcesBelongToAO(ctx, parent.AccessObjectUID, resourceUIDs); err != nil {
		return nil, err
	}

	r := &domain.Role{
		UID:             uuid.New().String(),
		AccessObjectUID: parent.AccessObjectUID,
		ParentRoleUID:   parentRoleUID,
		ResourceUIDs:    resourceUIDs,
		Name:            name,
		DisplayName:     displayName,
		Description:     description,
		Permissions:     permissions,
		Attributes:      attrs,
		Labels:          labels,
	}

	if err := uc.roleRepo.Add(ctx, r); err != nil {
		return nil, fmt.Errorf("add child role: %w", err)
	}
	return r, nil
}

func (uc *RoleUseCase) validateResourcesBelongToAO(ctx context.Context, accessObjectUID string, resourceUIDs []string) error {
	for _, rUID := range resourceUIDs {
		res, err := uc.resourceRepo.GetByUID(ctx, rUID)
		if err != nil {
			return fmt.Errorf("resource %s: %w", rUID, err)
		}
		if res.AccessObjectUID != accessObjectUID {
			return fmt.Errorf("resource %s: %w", rUID, domain.ErrResourceNotInObject)
		}
	}
	return nil
}
