package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) ListResources(ctx context.Context, req *pb.ListResourcesRequest) (*pb.ListResourcesResponse, error) {
	logger := s.logger.WithMethod("ListResources")
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
	list, total, err := s.service.List(ctx, domain.ResourceFilter{
		AccessObjectUID: req.AccessObjectUid,
		ParentUID:       req.ParentUid,
		ResourceType:    req.ResourceType,
		Page:            page,
		PageSize:        size,
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	resp := &pb.ListResourcesResponse{
		Page: &pb.PageResponse{TotalCount: total},
	}
	for i := range list {
		resp.Resources = append(resp.Resources, toResourceProto(&list[i]))
	}
	return resp, nil
}
