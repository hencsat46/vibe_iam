package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) UpdateResource(ctx context.Context, req *pb.UpdateResourceRequest) (*pb.Resource, error) {
	logger := s.logger.WithMethod("UpdateResource")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.Uid == "" {
		logger.Warn("validation failed: uid is required")
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	r, err := s.service.Update(ctx, req.Uid, domain.ResourceUpdate{
		DisplayName: req.DisplayName,
		Description: req.Description,
		Attributes:  req.Attributes,
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return toResourceProto(r), nil
}
