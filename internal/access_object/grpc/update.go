package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) UpdateEnvironment(ctx context.Context, req *pb.UpdateEnvironmentRequest) (*pb.AccessObject, error) {
	logger := s.logger.WithMethod("UpdateEnvironment")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.AccessObjectUid == "" {
		logger.Warn("validation failed: access_object_uid is required")
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}

	ao, err := s.service.UpdateEnvironment(ctx, req.AccessObjectUid, domain.EnvironmentUpdate{
		DisplayName: req.DisplayName,
		Description: req.Description,
		Attributes:  req.Attributes,
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return toAccessObjectProto(ao), nil
}
