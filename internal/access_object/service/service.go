package service

import (
	"context"

	aogrpc "temp/internal/access_object/grpc"
	"temp/internal/domain"
	"temp/internal/pkg/logger"
)

type IPostgres interface {
	Create(ctx context.Context, ao *domain.AccessObject) error
	GetByUID(ctx context.Context, uid string) (*domain.AccessObject, error)
	List(ctx context.Context, f domain.AccessObjectFilter) ([]domain.AccessObject, int32, error)
	UpdateEnvironment(ctx context.Context, uid string, upd domain.EnvironmentUpdate) (*domain.AccessObject, error)
	Delete(ctx context.Context, uid string) error
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.AccessObject, int32, error)
	AddResource(ctx context.Context, r *domain.Resource) error
	GetResourcePath(ctx context.Context, uid string) (string, error)
	AddRole(ctx context.Context, r *domain.Role) error
}

type service struct {
	postgres IPostgres
	logger   *logger.Logger
}

func NewService(postgres IPostgres, log *logger.Logger) aogrpc.IService {
	return &service{
		postgres: postgres,
		logger:   log.WithContext("service", "access_object"),
	}
}
