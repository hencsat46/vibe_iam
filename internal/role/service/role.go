package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"temp/internal/domain"
	"temp/internal/pkg/uid"
)

func (s *service) Add(ctx context.Context, r *domain.Role) (*domain.Role, error) {
	logger := s.logger.WithMethod("Add")
	logger.Info("Entering...")

	ao, err := s.accessObjectRepo.GetByUID(ctx, r.AccessObjectUID)
	if err != nil {
		logger.Error("couldn't get access object", zap.Error(err))
		return nil, err
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		logger.Warn("access object is not in DRAFT status")
		return nil, domain.ErrAccessObjectDraft
	}

	if err := s.validateResourcesBelongToAO(ctx, r.AccessObjectUID, r.ResourceUIDs); err != nil {
		logger.Warn("resource validation failed", zap.Error(err))
		return nil, err
	}

	r.UID = uid.New(uid.TypeRole, ao.Environment.SystemID, ao.Environment.Name)

	if err := s.postgres.Add(ctx, r); err != nil {
		logger.Error("couldn't add role", zap.Error(err))
		return nil, err
	}
	return r, nil
}

func (s *service) Get(ctx context.Context, uid string) (*domain.Role, error) {
	logger := s.logger.WithMethod("Get")
	logger.Info("Entering...", zap.String("uid", uid))

	r, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get role", zap.Error(err))
		return nil, err
	}
	return r, nil
}

func (s *service) List(ctx context.Context, f domain.RoleFilter) ([]domain.Role, int32, error) {
	logger := s.logger.WithMethod("List")
	logger.Info("Entering...")

	if f.PageSize <= 0 {
		f.PageSize = 20
	}

	list, total, err := s.postgres.List(ctx, f)
	if err != nil {
		logger.Error("couldn't list roles", zap.Error(err))
		return nil, 0, err
	}
	return list, total, nil
}

func (s *service) Update(ctx context.Context, uid string, upd domain.RoleUpdate) (*domain.Role, error) {
	logger := s.logger.WithMethod("Update")
	logger.Info("Entering...", zap.String("uid", uid))

	existing, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get role", zap.Error(err))
		return nil, err
	}

	ao, err := s.accessObjectRepo.GetByUID(ctx, existing.AccessObjectUID)
	if err != nil {
		logger.Error("couldn't get access object", zap.Error(err))
		return nil, err
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		logger.Warn("access object is not in DRAFT status")
		return nil, domain.ErrAccessObjectDraft
	}

	if err := s.validateResourcesBelongToAO(ctx, existing.AccessObjectUID, upd.ResourceUIDs); err != nil {
		logger.Warn("resource validation failed", zap.Error(err))
		return nil, err
	}

	r, err := s.postgres.Update(ctx, uid, upd)
	if err != nil {
		logger.Error("couldn't update role", zap.Error(err))
		return nil, err
	}
	return r, nil
}

func (s *service) Remove(ctx context.Context, uid string) error {
	logger := s.logger.WithMethod("Remove")
	logger.Info("Entering...", zap.String("uid", uid))

	existing, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get role", zap.Error(err))
		return err
	}

	ao, err := s.accessObjectRepo.GetByUID(ctx, existing.AccessObjectUID)
	if err != nil {
		logger.Error("couldn't get access object", zap.Error(err))
		return err
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		logger.Warn("access object is not in DRAFT status")
		return domain.ErrAccessObjectDraft
	}

	if err := s.postgres.Remove(ctx, uid); err != nil {
		logger.Error("couldn't remove role", zap.Error(err))
		return err
	}
	return nil
}

func (s *service) AddChild(ctx context.Context, parentRoleUID string, r *domain.Role) (*domain.Role, error) {
	logger := s.logger.WithMethod("AddChild")
	logger.Info("Entering...", zap.String("parent_role_uid", parentRoleUID))

	parent, err := s.postgres.GetByUID(ctx, parentRoleUID)
	if err != nil {
		logger.Error("couldn't get parent role", zap.Error(err))
		return nil, err
	}

	ao, err := s.accessObjectRepo.GetByUID(ctx, parent.AccessObjectUID)
	if err != nil {
		logger.Error("couldn't get access object", zap.Error(err))
		return nil, err
	}
	if ao.Lifecycle.Status != domain.LifecycleStatusDraft {
		logger.Warn("access object is not in DRAFT status")
		return nil, domain.ErrAccessObjectDraft
	}

	if err := s.validateResourcesBelongToAO(ctx, parent.AccessObjectUID, r.ResourceUIDs); err != nil {
		logger.Warn("resource validation failed", zap.Error(err))
		return nil, err
	}

	r.UID = uid.New(uid.TypeRole, ao.Environment.SystemID, ao.Environment.Name)
	r.AccessObjectUID = parent.AccessObjectUID
	r.ParentRoleUID = parentRoleUID

	if err := s.postgres.Add(ctx, r); err != nil {
		logger.Error("couldn't add child role", zap.Error(err))
		return nil, err
	}
	return r, nil
}

func (s *service) validateResourcesBelongToAO(ctx context.Context, accessObjectUID string, resourceUIDs []string) error {
	for _, rUID := range resourceUIDs {
		res, err := s.resourceRepo.GetByUID(ctx, rUID)
		if err != nil {
			return fmt.Errorf("resource %s: %w", rUID, err)
		}
		if res.AccessObjectUID != accessObjectUID {
			return fmt.Errorf("resource %s: %w", rUID, domain.ErrResourceNotInObject)
		}
	}
	return nil
}
