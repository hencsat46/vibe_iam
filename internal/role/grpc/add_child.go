package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) AddChildRole(ctx context.Context, req *pb.AddChildRoleRequest) (*pb.Role, error) {
	logger := s.logger.WithMethod("AddChildRole")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.ParentRoleUid == "" {
		logger.Warn("validation failed: parent_role_uid is required")
		return nil, status.Error(codes.InvalidArgument, "parent_role_uid is required")
	}
	if req.Name == "" {
		logger.Warn("validation failed: name is required")
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	r, err := s.service.AddChild(ctx, req.ParentRoleUid, &domain.Role{
		ResourceUIDs: req.ResourceUids,
		Name:         req.Name,
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
