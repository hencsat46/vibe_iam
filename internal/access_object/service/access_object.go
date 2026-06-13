package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	accessobject "temp/internal/access_object"
	"temp/internal/domain"
)

func (s *service) Create(ctx context.Context, req *accessobject.CreateRequest) (*domain.AccessObject, error) {
	logger := s.logger.WithMethod("Create")
	logger.Info("Entering...")

	aoUID := uuid.New().String()
	envUID := uuid.New().String()

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

	tempToUID := map[string]string{}
	if err := s.createResources(ctx, aoUID, req.Resources, tempToUID); err != nil {
		logger.Error("couldn't create resources", zap.Error(err))
		return nil, err
	}
	if err := s.createRoles(ctx, aoUID, "", req.Roles, tempToUID); err != nil {
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

func (s *service) createResources(ctx context.Context, aoUID string, inputs []accessobject.ResourceInput, tempToUID map[string]string) error {
	type node struct {
		input    accessobject.ResourceInput
		children []*node
	}

	nodes := make(map[string]*node, len(inputs))
	for i := range inputs {
		inp := inputs[i]
		nodes[inp.TempID] = &node{input: inp}
	}

	var roots []*node
	for i := range inputs {
		inp := inputs[i]
		n := nodes[inp.TempID]
		if inp.ParentTempID == "" {
			roots = append(roots, n)
		} else if parent, ok := nodes[inp.ParentTempID]; ok {
			parent.children = append(parent.children, n)
		}
	}

	var createNode func(n *node, parentUID string) error
	createNode = func(n *node, parentUID string) error {
		var path string
		if parentUID == "" {
			path = sanitizeLtree(n.input.Name)
		} else {
			parentPath, err := s.postgres.GetResourcePath(ctx, parentUID)
			if err != nil {
				return fmt.Errorf("get parent path: %w", err)
			}
			path = parentPath + "." + sanitizeLtree(n.input.Name)
		}

		r := &domain.Resource{
			UID:             uuid.New().String(),
			AccessObjectUID: aoUID,
			ParentUID:       parentUID,
			ResourceType:    n.input.ResourceType,
			Name:            n.input.Name,
			DisplayName:     n.input.DisplayName,
			Description:     n.input.Description,
			Path:            path,
			Attributes:      n.input.Attributes,
		}

		if err := s.postgres.AddResource(ctx, r); err != nil {
			return fmt.Errorf("add resource %q: %w", n.input.Name, err)
		}
		tempToUID[n.input.TempID] = r.UID

		for _, child := range n.children {
			if err := createNode(child, r.UID); err != nil {
				return err
			}
		}
		return nil
	}

	for _, root := range roots {
		if err := createNode(root, ""); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) createRoles(ctx context.Context, aoUID, parentRoleUID string, inputs []accessobject.RoleInput, tempToUID map[string]string) error {
	for _, inp := range inputs {
		var resourceUIDs []string
		for _, tempID := range inp.ResourceTempIDs {
			uid, ok := tempToUID[tempID]
			if !ok {
				return fmt.Errorf("resource temp_id %q not found", tempID)
			}
			resourceUIDs = append(resourceUIDs, uid)
		}

		r := &domain.Role{
			UID:             uuid.New().String(),
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

		if err := s.createRoles(ctx, aoUID, r.UID, inp.Children, tempToUID); err != nil {
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
