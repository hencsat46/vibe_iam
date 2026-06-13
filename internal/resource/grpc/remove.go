package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
)

func (s *Server) RemoveResource(ctx context.Context, req *pb.RemoveResourceRequest) (*pb.RemoveResourceResponse, error) {
	logger := s.logger.WithMethod("RemoveResource")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.Uid == "" {
		logger.Warn("validation failed: uid is required")
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	if err := s.service.Remove(ctx, req.Uid); err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &pb.RemoveResourceResponse{}, nil
}
