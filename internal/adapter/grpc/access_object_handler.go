package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
	"temp/internal/usecase"
)

type AccessObjectHandler struct {
	pb.UnimplementedAccessObjectServiceServer
	uc *usecase.AccessObjectUseCase
}

func NewAccessObjectHandler(uc *usecase.AccessObjectUseCase) *AccessObjectHandler {
	return &AccessObjectHandler{uc: uc}
}

func (h *AccessObjectHandler) CreateAccessObject(ctx context.Context, req *pb.CreateAccessObjectRequest) (*pb.AccessObject, error) {
	if req.SystemId == "" || req.EnvironmentName == "" {
		return nil, status.Error(codes.InvalidArgument, "system_id and environment_name are required")
	}

	ao, err := h.uc.Create(ctx,
		req.SystemId,
		req.EnvironmentName,
		req.DisplayName,
		req.Description,
		req.Attributes,
	)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainAOToProto(ao), nil
}

func (h *AccessObjectHandler) GetAccessObject(ctx context.Context, req *pb.GetAccessObjectRequest) (*pb.AccessObject, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	ao, err := h.uc.Get(ctx, req.Uid)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainAOToProto(ao), nil
}

func (h *AccessObjectHandler) ListAccessObjects(ctx context.Context, req *pb.ListAccessObjectsRequest) (*pb.ListAccessObjectsResponse, error) {
	page, size := pageRequestFromProto(req.Page)
	list, total, err := h.uc.List(ctx, domain.AccessObjectFilter{
		SystemID: req.SystemId,
		Status:   req.Status,
		Page:     page,
		PageSize: size,
	})
	if err != nil {
		return nil, domainErrToGRPC(err)
	}

	resp := &pb.ListAccessObjectsResponse{
		Page: &pb.PageResponse{
			TotalCount: total,
		},
	}
	for _, ao := range list {
		ao := ao
		resp.AccessObjects = append(resp.AccessObjects, domainAOToProto(&ao))
	}
	return resp, nil
}

func (h *AccessObjectHandler) UpdateEnvironment(ctx context.Context, req *pb.UpdateEnvironmentRequest) (*pb.AccessObject, error) {
	if req.AccessObjectUid == "" {
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}

	ao, err := h.uc.UpdateEnvironment(ctx, req.AccessObjectUid, domain.EnvironmentUpdate{
		DisplayName: req.DisplayName,
		Description: req.Description,
		Attributes:  req.Attributes,
	})
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainAOToProto(ao), nil
}

func (h *AccessObjectHandler) DeleteAccessObject(ctx context.Context, req *pb.DeleteAccessObjectRequest) (*pb.DeleteAccessObjectResponse, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	if err := h.uc.Delete(ctx, req.Uid); err != nil {
		return nil, domainErrToGRPC(err)
	}
	return &pb.DeleteAccessObjectResponse{}, nil
}

func (h *AccessObjectHandler) SearchAccessObjects(ctx context.Context, req *pb.SearchAccessObjectsRequest) (*pb.SearchAccessObjectsResponse, error) {
	if req.Query == "" && req.SystemId == "" && req.ResourceType == "" && req.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "at least one search parameter is required")
	}

	page, size := pageRequestFromProto(req.Page)
	list, total, err := h.uc.Search(ctx, domain.SearchQuery{
		Query:        req.Query,
		SystemID:     req.SystemId,
		ResourceType: req.ResourceType,
		Status:       req.Status,
		Page:         page,
		PageSize:     size,
	})
	if err != nil {
		return nil, domainErrToGRPC(fmt.Errorf("search: %w", err))
	}

	resp := &pb.SearchAccessObjectsResponse{
		Page: &pb.PageResponse{TotalCount: total},
	}
	for _, ao := range list {
		ao := ao
		resp.AccessObjects = append(resp.AccessObjects, domainAOToProto(&ao))
	}
	return resp, nil
}
