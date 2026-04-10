package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"temp/internal/domain"

	"github.com/google/uuid"
)

var nonLtreeChar = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeLtree(s string) string {
	s = strings.ToLower(s)
	s = nonLtreeChar.ReplaceAllString(s, "_")
	if s == "" {
		s = "x"
	}
	return s
}

type ResourceUseCase struct {
	resourceRepo     domain.ResourceRepository
	accessObjectRepo domain.AccessObjectRepository
}

func NewResourceUseCase(rr domain.ResourceRepository, aor domain.AccessObjectRepository) *ResourceUseCase {
	return &ResourceUseCase{resourceRepo: rr, accessObjectRepo: aor}
}

func (uc *ResourceUseCase) Add(ctx context.Context, accessObjectUID, parentUID, resourceType, name, displayName, description string, attrs map[string]string) (*domain.Resource, error) {
	ao, err := uc.accessObjectRepo.GetByUID(ctx, accessObjectUID)
	if err != nil {
		return nil, fmt.Errorf("get access object: %w", err)
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		return nil, domain.ErrAccessObjectDraft
	}

	var path string
	if parentUID == "" {
		path = sanitizeLtree(name)
	} else {
		parentPath, err := uc.resourceRepo.GetPath(ctx, parentUID)
		if err != nil {
			return nil, fmt.Errorf("get parent path: %w", err)
		}
		path = parentPath + "." + sanitizeLtree(name)
	}

	r := &domain.Resource{
		UID:             uuid.New().String(),
		AccessObjectUID: accessObjectUID,
		ParentUID:       parentUID,
		ResourceType:    resourceType,
		Name:            name,
		DisplayName:     displayName,
		Description:     description,
		Path:            path,
		Attributes:      attrs,
	}

	if err := uc.resourceRepo.Add(ctx, r); err != nil {
		return nil, fmt.Errorf("add resource: %w", err)
	}
	return r, nil
}

func (uc *ResourceUseCase) Get(ctx context.Context, uid string) (*domain.Resource, error) {
	r, err := uc.resourceRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get resource: %w", err)
	}
	return r, nil
}

func (uc *ResourceUseCase) List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, int32, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	return uc.resourceRepo.List(ctx, f)
}

func (uc *ResourceUseCase) Update(ctx context.Context, uid string, upd domain.ResourceUpdate) (*domain.Resource, error) {
	existing, err := uc.resourceRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get resource: %w", err)
	}

	ao, err := uc.accessObjectRepo.GetByUID(ctx, existing.AccessObjectUID)
	if err != nil {
		return nil, fmt.Errorf("get access object: %w", err)
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		return nil, domain.ErrAccessObjectDraft
	}

	r, err := uc.resourceRepo.Update(ctx, uid, upd)
	if err != nil {
		return nil, fmt.Errorf("update resource: %w", err)
	}
	return r, nil
}

func (uc *ResourceUseCase) Remove(ctx context.Context, uid string) error {
	existing, err := uc.resourceRepo.GetByUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("get resource: %w", err)
	}

	ao, err := uc.accessObjectRepo.GetByUID(ctx, existing.AccessObjectUID)
	if err != nil {
		return fmt.Errorf("get access object: %w", err)
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		return domain.ErrAccessObjectDraft
	}

	if err := uc.resourceRepo.Remove(ctx, uid); err != nil {
		return fmt.Errorf("remove resource: %w", err)
	}
	return nil
}

func (uc *ResourceUseCase) GetSubtree(ctx context.Context, uid string, maxDepth int32) (*domain.Resource, []domain.Resource, error) {
	root, children, err := uc.resourceRepo.GetSubtree(ctx, uid, maxDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("get subtree: %w", err)
	}
	return root, children, nil
}
