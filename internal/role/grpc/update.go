package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) UpdateRole(ctx context.Context, req *pb.UpdateRoleRequest) (*pb.Role, error) {
	logger := s.logger.WithMethod("UpdateRole")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.Uid == "" {
		logger.Warn("validation failed: uid is required")
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	r, err := s.service.Update(ctx, req.Uid, domain.RoleUpdate{
		ResourceUIDs: req.ResourceUids,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		Permissions:  req.Permissions,
		Attributes:   req.Attributes,
		Labels:       toLabels(req.Labels),
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return toRoleProto(r), nil
}
