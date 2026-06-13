package grpc

import (
	"context"

	pb "temp/gen/pb/v1"
	accessobject "temp/internal/access_object"
	"temp/internal/domain"
	"temp/internal/pkg/logger"
)

type Server struct {
	pb.UnimplementedAccessObjectServiceServer
	service IService
	logger  *logger.Logger
}

type IService interface {
	Create(ctx context.Context, req *accessobject.CreateRequest) (*domain.AccessObject, error)
	Get(ctx context.Context, uid string) (*domain.AccessObject, error)
	List(ctx context.Context, f domain.AccessObjectFilter) ([]domain.AccessObject, int32, error)
	UpdateEnvironment(ctx context.Context, uid string, upd domain.EnvironmentUpdate) (*domain.AccessObject, error)
	Delete(ctx context.Context, uid string) error
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.AccessObject, int32, error)
}

func NewServer(service IService, log *logger.Logger) *Server {
	return &Server{
		service: service,
		logger:  log.WithContext("grpc", "access_object"),
	}
}
