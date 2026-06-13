package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) AddResource(ctx context.Context, req *pb.AddResourceRequest) (*pb.Resource, error) {
	logger := s.logger.WithMethod("AddResource")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.AccessObjectUid == "" {
		logger.Warn("validation failed: access_object_uid is required")
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}
	if req.Name == "" {
		logger.Warn("validation failed: name is required")
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.ResourceType == "" {
		logger.Warn("validation failed: resource_type is required")
		return nil, status.Error(codes.InvalidArgument, "resource_type is required")
	}

	r, err := s.service.Add(ctx, &domain.Resource{
		AccessObjectUID: req.AccessObjectUid,
		ParentUID:       req.ParentUid,
		ResourceType:    req.ResourceType,
		Name:            req.Name,
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		Attributes:      req.Attributes,
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return toResourceProto(r), nil
}
