package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"temp/internal/domain"
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

func (s *service) Add(ctx context.Context, r *domain.Resource) (*domain.Resource, error) {
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

	var path string
	if r.ParentUID == "" {
		path = sanitizeLtree(r.Name)
	} else {
		parentPath, err := s.postgres.GetPath(ctx, r.ParentUID)
		if err != nil {
			logger.Error("couldn't get parent path", zap.Error(err))
			return nil, err
		}
		path = parentPath + "." + sanitizeLtree(r.Name)
	}

	r.UID = uuid.New().String()
	r.Path = path

	if err := s.postgres.Add(ctx, r); err != nil {
		logger.Error("couldn't add resource", zap.Error(err))
		return nil, err
	}
	return r, nil
}

func (s *service) Get(ctx context.Context, uid string) (*domain.Resource, error) {
	logger := s.logger.WithMethod("Get")
	logger.Info("Entering...", zap.String("uid", uid))

	r, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get resource", zap.Error(err))
		return nil, err
	}
	return r, nil
}

func (s *service) List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, int32, error) {
	logger := s.logger.WithMethod("List")
	logger.Info("Entering...")

	if f.PageSize <= 0 {
		f.PageSize = 20
	}

	list, total, err := s.postgres.List(ctx, f)
	if err != nil {
		logger.Error("couldn't list resources", zap.Error(err))
		return nil, 0, err
	}
	return list, total, nil
}

func (s *service) Update(ctx context.Context, uid string, upd domain.ResourceUpdate) (*domain.Resource, error) {
	logger := s.logger.WithMethod("Update")
	logger.Info("Entering...", zap.String("uid", uid))

	existing, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get resource", zap.Error(err))
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

	r, err := s.postgres.Update(ctx, uid, upd)
	if err != nil {
		logger.Error("couldn't update resource", zap.Error(err))
		return nil, err
	}
	return r, nil
}

func (s *service) Remove(ctx context.Context, uid string) error {
	logger := s.logger.WithMethod("Remove")
	logger.Info("Entering...", zap.String("uid", uid))

	existing, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get resource", zap.Error(err))
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
		logger.Error("couldn't remove resource", zap.Error(err))
		return err
	}
	return nil
}

func (s *service) GetSubtree(ctx context.Context, uid string, maxDepth int32) (*domain.Resource, []domain.Resource, error) {
	logger := s.logger.WithMethod("GetSubtree")
	logger.Info("Entering...", zap.String("uid", uid))

	root, children, err := s.postgres.GetSubtree(ctx, uid, maxDepth)
	if err != nil {
		logger.Error("couldn't get subtree", zap.Error(err))
		return nil, nil, err
	}
	return root, children, nil
}
