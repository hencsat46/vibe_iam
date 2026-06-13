package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
)

func (s *Server) GetRole(ctx context.Context, req *pb.GetRoleRequest) (*pb.Role, error) {
	logger := s.logger.WithMethod("GetRole")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.Uid == "" {
		logger.Warn("validation failed: uid is required")
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	r, err := s.service.Get(ctx, req.Uid)
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return toRoleProto(r), nil
}
