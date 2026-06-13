package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
)

func (s *Server) CreateAccessObject(ctx context.Context, req *pb.CreateAccessObjectRequest) (*pb.AccessObject, error) {
	logger := s.logger.WithMethod("CreateAccessObject")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.SystemId == "" {
		logger.Warn("validation failed: system_id is required")
		return nil, status.Error(codes.InvalidArgument, "system_id is required")
	}
	if req.EnvironmentName == "" {
		logger.Warn("validation failed: environment_name is required")
		return nil, status.Error(codes.InvalidArgument, "environment_name is required")
	}

	createReq := toCreateRequest(req)

	result, err := s.service.Create(ctx, createReq)
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return toAccessObjectProto(result), nil
}
