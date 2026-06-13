package service

import (
	"context"

	resgrpc "temp/internal/resource/grpc"
	"temp/internal/domain"
	"temp/internal/pkg/logger"
)

type IPostgres interface {
	Add(ctx context.Context, r *domain.Resource) error
	GetByUID(ctx context.Context, uid string) (*domain.Resource, error)
	List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, int32, error)
	Update(ctx context.Context, uid string, upd domain.ResourceUpdate) (*domain.Resource, error)
	Remove(ctx context.Context, uid string) error
	GetSubtree(ctx context.Context, uid string, maxDepth int32) (*domain.Resource, []domain.Resource, error)
	GetPath(ctx context.Context, uid string) (string, error)
}

type IAccessObjectPostgres interface {
	GetByUID(ctx context.Context, uid string) (*domain.AccessObject, error)
}

type service struct {
	postgres         IPostgres
	accessObjectRepo IAccessObjectPostgres
	logger           *logger.Logger
}

func NewService(postgres IPostgres, accessObjectRepo IAccessObjectPostgres, log *logger.Logger) resgrpc.IService {
	return &service{
		postgres:         postgres,
		accessObjectRepo: accessObjectRepo,
		logger:           log.WithContext("service", "resource"),
	}
}
