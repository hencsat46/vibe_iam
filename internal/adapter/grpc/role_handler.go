package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
	"temp/internal/usecase"
)

type RoleHandler struct {
	pb.UnimplementedRoleServiceServer
	uc *usecase.RoleUseCase
}

func NewRoleHandler(uc *usecase.RoleUseCase) *RoleHandler {
	return &RoleHandler{uc: uc}
}

func (h *RoleHandler) AddRole(ctx context.Context, req *pb.AddRoleRequest) (*pb.Role, error) {
	if req.AccessObjectUid == "" {
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	r, err := h.uc.Add(ctx,
		req.AccessObjectUid,
		req.ResourceUids,
		req.Name,
		req.DisplayName,
		req.Description,
		req.Permissions,
		req.Attributes,
		protoLabelsToDomain(req.Labels),
	)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainRoleToProto(r), nil
}

func (h *RoleHandler) GetRole(ctx context.Context, req *pb.GetRoleRequest) (*pb.Role, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	r, err := h.uc.Get(ctx, req.Uid)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainRoleToProto(r), nil
}

func (h *RoleHandler) ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.ListRolesResponse, error) {
	if req.AccessObjectUid == "" {
		return nil, status.Error(codes.InvalidArgument, "access_object_uid is required")
	}

	page, size := pageRequestFromProto(req.Page)
	list, total, err := h.uc.List(ctx, domain.RoleFilter{
		AccessObjectUID: req.AccessObjectUid,
		ResourceUID:     req.ResourceUid,
		Page:            page,
		PageSize:        size,
	})
	if err != nil {
		return nil, domainErrToGRPC(err)
	}

	resp := &pb.ListRolesResponse{
		Page: &pb.PageResponse{TotalCount: total},
	}
	for _, r := range list {
		r := r
		resp.Roles = append(resp.Roles, domainRoleToProto(&r))
	}
	return resp, nil
}

func (h *RoleHandler) UpdateRole(ctx context.Context, req *pb.UpdateRoleRequest) (*pb.Role, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	r, err := h.uc.Update(ctx, req.Uid, domain.RoleUpdate{
		ResourceUIDs: req.ResourceUids,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		Permissions:  req.Permissions,
		Attributes:   req.Attributes,
		Labels:       protoLabelsToDomain(req.Labels),
	})
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainRoleToProto(r), nil
}

func (h *RoleHandler) RemoveRole(ctx context.Context, req *pb.RemoveRoleRequest) (*pb.RemoveRoleResponse, error) {
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}

	if err := h.uc.Remove(ctx, req.Uid); err != nil {
		return nil, domainErrToGRPC(err)
	}
	return &pb.RemoveRoleResponse{}, nil
}

func (h *RoleHandler) AddChildRole(ctx context.Context, req *pb.AddChildRoleRequest) (*pb.Role, error) {
	if req.ParentRoleUid == "" {
		return nil, status.Error(codes.InvalidArgument, "parent_role_uid is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	r, err := h.uc.AddChildRole(ctx,
		req.ParentRoleUid,
		req.ResourceUids,
		req.Name,
		req.DisplayName,
		req.Description,
		req.Permissions,
		req.Attributes,
		protoLabelsToDomain(req.Labels),
	)
	if err != nil {
		return nil, domainErrToGRPC(err)
	}
	return domainRoleToProto(r), nil
}
