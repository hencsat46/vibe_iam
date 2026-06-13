package grpc

import (
	"context"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
	"temp/internal/pkg/logger"
)

type Server struct {
	pb.UnimplementedResourceServiceServer
	service IService
	logger  *logger.Logger
}

type IService interface {
	Add(ctx context.Context, r *domain.Resource) (*domain.Resource, error)
	Get(ctx context.Context, uid string) (*domain.Resource, error)
	List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, int32, error)
	Update(ctx context.Context, uid string, upd domain.ResourceUpdate) (*domain.Resource, error)
	Remove(ctx context.Context, uid string) error
	GetSubtree(ctx context.Context, uid string, maxDepth int32) (*domain.Resource, []domain.Resource, error)
}

func NewServer(service IService, log *logger.Logger) *Server {
	return &Server{
		service: service,
		logger:  log.WithContext("grpc", "resource"),
	}
}
