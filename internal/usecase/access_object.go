package usecase

import (
	"context"
	"fmt"

	"temp/internal/domain"

	"github.com/google/uuid"
)

type AccessObjectUseCase struct {
	repo domain.AccessObjectRepository
}

func NewAccessObjectUseCase(repo domain.AccessObjectRepository) *AccessObjectUseCase {
	return &AccessObjectUseCase{repo: repo}
}

func (uc *AccessObjectUseCase) Create(ctx context.Context, systemID, envName, displayName, description string, attrs map[string]string) (*domain.AccessObject, error) {
	if systemID == "" {
		return nil, fmt.Errorf("system_id is required")
	}
	if envName == "" {
		return nil, fmt.Errorf("environment_name is required")
	}

	aoUID := uuid.New().String()
	envUID := uuid.New().String()

	ao := &domain.AccessObject{
		UID: aoUID,
		Environment: domain.Environment{
			UID:         envUID,
			SystemID:    systemID,
			Name:        envName,
			DisplayName: displayName,
			Description: description,
			Attributes:  attrs,
		},
		Lifecycle: domain.Lifecycle{
			Status:  domain.LifecycleStatusDraft,
			Version: 1,
		},
	}

	if err := uc.repo.Create(ctx, ao); err != nil {
		return nil, fmt.Errorf("create access object: %w", err)
	}
	return ao, nil
}

func (uc *AccessObjectUseCase) Get(ctx context.Context, uid string) (*domain.AccessObject, error) {
	ao, err := uc.repo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get access object: %w", err)
	}
	return ao, nil
}

func (uc *AccessObjectUseCase) List(ctx context.Context, f domain.AccessObjectFilter) ([]domain.AccessObject, int32, error) {
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	return uc.repo.List(ctx, f)
}

func (uc *AccessObjectUseCase) UpdateEnvironment(ctx context.Context, uid string, upd domain.EnvironmentUpdate) (*domain.AccessObject, error) {
	if uid == "" {
		return nil, fmt.Errorf("access_object_uid is required")
	}
	ao, err := uc.repo.UpdateEnvironment(ctx, uid, upd)
	if err != nil {
		return nil, fmt.Errorf("update environment: %w", err)
	}
	return ao, nil
}

func (uc *AccessObjectUseCase) Delete(ctx context.Context, uid string) error {
	ao, err := uc.repo.GetByUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("get access object: %w", err)
	}

	s := ao.Lifecycle.Status
	if s != domain.LifecycleStatusDraft && s != domain.LifecycleStatusRetired {
		return domain.ErrDeleteRestricted
	}

	if err := uc.repo.Delete(ctx, uid); err != nil {
		return fmt.Errorf("delete access object: %w", err)
	}
	return nil
}

func (uc *AccessObjectUseCase) Search(ctx context.Context, q domain.SearchQuery) ([]domain.AccessObject, int32, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	return uc.repo.Search(ctx, q)
}
