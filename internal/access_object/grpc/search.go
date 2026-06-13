package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) SearchAccessObjects(ctx context.Context, req *pb.SearchAccessObjectsRequest) (*pb.SearchAccessObjectsResponse, error) {
	logger := s.logger.WithMethod("SearchAccessObjects")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	if req.Query == "" && req.SystemId == "" && req.ResourceType == "" && req.Status == "" {
		logger.Warn("validation failed: at least one search parameter is required")
		return nil, status.Error(codes.InvalidArgument, "at least one search parameter is required")
	}

	page, size := pageFromProto(req.Page)
	list, total, err := s.service.Search(ctx, domain.SearchQuery{
		Query:        req.Query,
		SystemID:     req.SystemId,
		ResourceType: req.ResourceType,
		Status:       req.Status,
		Page:         page,
		PageSize:     size,
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	resp := &pb.SearchAccessObjectsResponse{
		Page: &pb.PageResponse{TotalCount: total},
	}
	for i := range list {
		resp.AccessObjects = append(resp.AccessObjects, toAccessObjectProto(&list[i]))
	}
	return resp, nil
}
