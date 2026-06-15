package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	accessobject "temp/internal/access_object"
	"temp/internal/domain"
	"temp/internal/pkg/uid"
)

func (s *service) Create(ctx context.Context, req *accessobject.CreateRequest) (*domain.AccessObject, error) {
	logger := s.logger.WithMethod("Create")
	logger.Info("Entering...")

	aoUID := uid.New(uid.TypeAccessObject, req.SystemID, req.EnvironmentName)
	envUID := uid.New(uid.TypeEnvironment, req.SystemID, req.EnvironmentName)

	ao := &domain.AccessObject{
		UID: aoUID,
		Environment: domain.Environment{
			UID:         envUID,
			SystemID:    req.SystemID,
			Name:        req.EnvironmentName,
			DisplayName: req.DisplayName,
			Description: req.Description,
			Attributes:  req.Attributes,
		},
		Lifecycle: domain.Lifecycle{
			Status:  domain.LifecycleStatusDraft,
			Version: 1,
		},
	}

	if err := s.postgres.Create(ctx, ao); err != nil {
		logger.Error("couldn't create access object", zap.Error(err))
		return nil, err
	}

	nameToUID := map[string]string{}
	if err := s.createResources(ctx, aoUID, req.SystemID, req.EnvironmentName, req.Resources, nameToUID); err != nil {
		logger.Error("couldn't create resources", zap.Error(err))
		return nil, err
	}
	if err := s.createRoles(ctx, aoUID, req.SystemID, req.EnvironmentName, "", req.Roles, nameToUID); err != nil {
		logger.Error("couldn't create roles", zap.Error(err))
		return nil, err
	}

	result, err := s.postgres.GetByUID(ctx, aoUID)
	if err != nil {
		logger.Error("couldn't fetch created access object", zap.Error(err))
		return nil, err
	}
	return result, nil
}

func (s *service) createResources(ctx context.Context, aoUID, systemID, envName string, inputs []accessobject.ResourceInput, nameToUID map[string]string) error {
	return s.createResourceNodes(ctx, aoUID, systemID, envName, inputs, "", "", nameToUID)
}

func (s *service) createResourceNodes(ctx context.Context, aoUID, systemID, envName string, inputs []accessobject.ResourceInput, parentUID, parentPath string, nameToUID map[string]string) error {
	for _, inp := range inputs {
		var path string
		if parentPath == "" {
			path = sanitizeLtree(inp.Name)
		} else {
			path = parentPath + "." + sanitizeLtree(inp.Name)
		}

		r := &domain.Resource{
			UID:             uid.New(uid.TypeResource, systemID, envName),
			AccessObjectUID: aoUID,
			ParentUID:       parentUID,
			ResourceType:    inp.ResourceType,
			Name:            inp.Name,
			DisplayName:     inp.DisplayName,
			Description:     inp.Description,
			Path:            path,
			Attributes:      inp.Attributes,
		}

		if err := s.postgres.AddResource(ctx, r); err != nil {
			return fmt.Errorf("add resource %q: %w", inp.Name, err)
		}

		nameToUID[inp.Name] = r.UID
		nameToUID[strings.ReplaceAll(path, ".", "/")] = r.UID

		if err := s.createResourceNodes(ctx, aoUID, systemID, envName, inp.Children, r.UID, path, nameToUID); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) createRoles(ctx context.Context, aoUID, systemID, envName, parentRoleUID string, inputs []accessobject.RoleInput, nameToUID map[string]string) error {
	for _, inp := range inputs {
		var resourceUIDs []string
		for _, name := range inp.ResourceNames {
			uid, ok := nameToUID[name]
			if !ok {
				return fmt.Errorf("resource %q not found", name)
			}
			resourceUIDs = append(resourceUIDs, uid)
		}

		r := &domain.Role{
			UID:             uid.New(uid.TypeRole, systemID, envName),
			AccessObjectUID: aoUID,
			ParentRoleUID:   parentRoleUID,
			ResourceUIDs:    resourceUIDs,
			Name:            inp.Name,
			DisplayName:     inp.DisplayName,
			Description:     inp.Description,
			Permissions:     inp.Permissions,
			Attributes:      inp.Attributes,
			Labels:          inp.Labels,
		}

		if err := s.postgres.AddRole(ctx, r); err != nil {
			return fmt.Errorf("add role %q: %w", inp.Name, err)
		}

		if err := s.createRoles(ctx, aoUID, systemID, envName, r.UID, inp.Children, nameToUID); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) Get(ctx context.Context, uid string) (*domain.AccessObject, error) {
	logger := s.logger.WithMethod("Get")
	logger.Info("Entering...")

	ao, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get access object", zap.Error(err))
		return nil, err
	}
	return ao, nil
}

func (s *service) List(ctx context.Context, f domain.AccessObjectFilter) ([]domain.AccessObject, int32, error) {
	logger := s.logger.WithMethod("List")
	logger.Info("Entering...")

	if f.PageSize <= 0 {
		f.PageSize = 20
	}

	list, total, err := s.postgres.List(ctx, f)
	if err != nil {
		logger.Error("couldn't list access objects", zap.Error(err))
		return nil, 0, err
	}
	return list, total, nil
}

func (s *service) UpdateEnvironment(ctx context.Context, uid string, upd domain.EnvironmentUpdate) (*domain.AccessObject, error) {
	logger := s.logger.WithMethod("UpdateEnvironment")
	logger.Info("Entering...")

	ao, err := s.postgres.UpdateEnvironment(ctx, uid, upd)
	if err != nil {
		logger.Error("couldn't update environment", zap.Error(err))
		return nil, err
	}
	return ao, nil
}

func (s *service) Delete(ctx context.Context, uid string) error {
	logger := s.logger.WithMethod("Delete")
	logger.Info("Entering...")

	ao, err := s.postgres.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get access object", zap.Error(err))
		return err
	}

	st := ao.Lifecycle.Status
	if st != domain.LifecycleStatusDraft && st != domain.LifecycleStatusRetired {
		logger.Warn("delete restricted: invalid lifecycle status")
		return domain.ErrDeleteRestricted
	}

	if err := s.postgres.Delete(ctx, uid); err != nil {
		logger.Error("couldn't delete access object", zap.Error(err))
		return err
	}
	return nil
}

func (s *service) Search(ctx context.Context, q domain.SearchQuery) ([]domain.AccessObject, int32, error) {
	logger := s.logger.WithMethod("Search")
	logger.Info("Entering...")

	if q.PageSize <= 0 {
		q.PageSize = 20
	}

	list, total, err := s.postgres.Search(ctx, q)
	if err != nil {
		logger.Error("couldn't search access objects", zap.Error(err))
		return nil, 0, err
	}
	return list, total, nil
}

var nonLtreeChar = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func sanitizeLtree(s string) string {
	s = strings.ToLower(s)
	s = nonLtreeChar.ReplaceAllString(s, "_")
	if s == "" {
		s = "x"
	}
	return s
}
