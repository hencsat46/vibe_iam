package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

func (s *Server) ListAccessObjects(ctx context.Context, req *pb.ListAccessObjectsRequest) (*pb.ListAccessObjectsResponse, error) {
	logger := s.logger.WithMethod("ListAccessObjects")
	logger.Info("Entering...")

	if ctx.Err() != nil {
		logger.Warn("request cancelled by the client", zap.Error(ctx.Err()))
		return nil, status.Error(codes.Canceled, "request cancelled by the client")
	}

	page, size := pageFromProto(req.Page)
	list, total, err := s.service.List(ctx, domain.AccessObjectFilter{
		SystemID: req.SystemId,
		Status:   req.Status,
		Page:     page,
		PageSize: size,
	})
	if err != nil {
		logger.Error("internal error", zap.Error(err))
		return nil, toGRPCError(err)
	}

	resp := &pb.ListAccessObjectsResponse{
		Page: &pb.PageResponse{TotalCount: total},
	}
	for i := range list {
		resp.AccessObjects = append(resp.AccessObjects, toAccessObjectProto(&list[i]))
	}
	return resp, nil
}
