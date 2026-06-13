package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.ListRolesResponse, error) {
	logger := s.logger.WithMethod("ListRoles")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.AccessObjectUid == "" {
		logger.Warn("validation failed: access_object_uid is required")
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}

	page, size := pageFromProto(req.Page)
	list, total, err := s.service.List(ctx, domain.RoleFilter{
		AccessObjectUID: req.AccessObjectUid,
		ResourceUID:     req.ResourceUid,
		Page:            page,
		PageSize:        size,
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	resp := &pb.ListRolesResponse{
		Page: &pb.PageResponse{TotalCount: total},
	}
	for i := range list {
		resp.Roles = append(resp.Roles, toRoleProto(&list[i]))
	}
	return resp, nil
}
