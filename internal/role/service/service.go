package service

import (
	"context"

	rolegrpc "temp/internal/role/grpc"
	"temp/internal/domain"
	"temp/internal/pkg/logger"
)

type IPostgres interface {
	Add(ctx context.Context, r *domain.Role) error
	GetByUID(ctx context.Context, uid string) (*domain.Role, error)
	List(ctx context.Context, f domain.RoleFilter) ([]domain.Role, int32, error)
	Update(ctx context.Context, uid string, upd domain.RoleUpdate) (*domain.Role, error)
	Remove(ctx context.Context, uid string) error
}

type IAccessObjectPostgres interface {
	GetByUID(ctx context.Context, uid string) (*domain.AccessObject, error)
}

type IResourcePostgres interface {
	GetByUID(ctx context.Context, uid string) (*domain.Resource, error)
}

type service struct {
	postgres         IPostgres
	accessObjectRepo IAccessObjectPostgres
	resourceRepo     IResourcePostgres
	logger           *logger.Logger
}

func NewService(postgres IPostgres, accessObjectRepo IAccessObjectPostgres, resourceRepo IResourcePostgres, log *logger.Logger) rolegrpc.IService {
	return &service{
		postgres:         postgres,
		accessObjectRepo: accessObjectRepo,
		resourceRepo:     resourceRepo,
		logger:           log.WithContext("service", "role"),
	}
}
