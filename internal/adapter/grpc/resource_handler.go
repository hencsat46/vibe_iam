package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
	"temp/internal/usecase"
)

type ResourceHandler struct {
	pb.UnimplementedResourceServiceServer
	uc *usecase.ResourceUseCase
}

func NewResourceHandler(uc *usecase.ResourceUseCase) *ResourceHandler {
	return &ResourceHandler{uc: uc}
}

func (h *ResourceHandler) AddResource(ctx context.Context, req *pb.AddResourceRequest) (*pb.Resource, error) {
	if req.AccessObjectUid == "" {
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.ResourceType == "" {
		return nil, status.Error(codes.InvalidArgument, "resource_type is required")
	}

	r, err := h.uc.Add(ctx,
		req.AccessObjectUid,
		req.ParentUid,
		req.ResourceType,
		req.Name,
		req.DisplayName,
		req.Description,
		req.Attributes,
	)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainResourceToProto(r), nil
}

func (h *ResourceHandler) GetResource(ctx context.Context, req *pb.GetResourceRequest) (*pb.Resource, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	r, err := h.uc.Get(ctx, req.Uid)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainResourceToProto(r), nil
}

func (h *ResourceHandler) ListResources(ctx context.Context, req *pb.ListResourcesRequest) (*pb.ListResourcesResponse, error) {
	if req.AccessObjectUid == "" {
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}

	page, size := pageRequestFromProto(req.Page)
	list, total, err := h.uc.List(ctx, domain.ResourceFilter{
		AccessObjectUID: req.AccessObjectUid,
		ParentUID:       req.ParentUid,
		ResourceType:    req.ResourceType,
		Page:            page,
		PageSize:        size,
	})
	if err != nil {
		return nil, domainErrToGRPC(err)
	}

	resp := &pb.ListResourcesResponse{
		Page: &pb.PageResponse{TotalCount: total},
	}
	for _, r := range list {
		r := r
		resp.Resources = append(resp.Resources, domainResourceToProto(&r))
	}
	return resp, nil
}

func (h *ResourceHandler) UpdateResource(ctx context.Context, req *pb.UpdateResourceRequest) (*pb.Resource, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	r, err := h.uc.Update(ctx, req.Uid, domain.ResourceUpdate{
		DisplayName: req.DisplayName,
		Description: req.Description,
		Attributes:  req.Attributes,
	})
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainResourceToProto(r), nil
}

func (h *ResourceHandler) RemoveResource(ctx context.Context, req *pb.RemoveResourceRequest) (*pb.RemoveResourceResponse, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	if err := h.uc.Remove(ctx, req.Uid); err != nil {
		return nil, domainErrToGRPC(err)
	}
	return &pb.RemoveResourceResponse{}, nil
}

func (h *ResourceHandler) GetSubtree(ctx context.Context, req *pb.GetSubtreeRequest) (*pb.GetSubtreeResponse, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	root, children, err := h.uc.GetSubtree(ctx, req.Uid, req.MaxDepth)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}

	resp := &pb.GetSubtreeResponse{
		Root: domainResourceToProto(root),
	}
	for _, c := range children {
		c := c
		resp.Children = append(resp.Children, domainResourceToProto(&c))
	}
	return resp, nil
}
