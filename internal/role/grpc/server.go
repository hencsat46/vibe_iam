package grpc

import (
	"context"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
	"temp/internal/pkg/logger"
)

type Server struct {
	pb.UnimplementedRoleServiceServer
	service IService
	logger  *logger.Logger
}

type IService interface {
	Add(ctx context.Context, r *domain.Role) (*domain.Role, error)
	Get(ctx context.Context, uid string) (*domain.Role, error)
	List(ctx context.Context, f domain.RoleFilter) ([]domain.Role, int32, error)
	Update(ctx context.Context, uid string, upd domain.RoleUpdate) (*domain.Role, error)
	Remove(ctx context.Context, uid string) error
	AddChild(ctx context.Context, parentRoleUID string, r *domain.Role) (*domain.Role, error)
}

func NewServer(service IService, log *logger.Logger) *Server {
	return &Server{
		service: service,
		logger:  log.WithContext("grpc", "role"),
	}
}
