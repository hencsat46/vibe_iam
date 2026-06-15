package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "temp/gen/pb/v1"
	accessobject "temp/internal/access_object"
	"temp/internal/domain"
)

// ─── proto → entity ───

func toCreateRequest(req *pb.CreateAccessObjectRequest) *accessobject.CreateRequest {
	return &accessobject.CreateRequest{
		SystemID:        req.SystemId,
		EnvironmentName: req.EnvironmentName,
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		Attributes:      req.Attributes,
		Resources:       toResourceInputs(req.Resources),
		Roles:           toRoleInputs(req.Roles),
	}
}

func toResourceInputs(inputs []*pb.CreateResourceInput) []accessobject.ResourceInput {
	result := make([]accessobject.ResourceInput, len(inputs))
	for i, inp := range inputs {
		result[i] = accessobject.ResourceInput{
			ResourceType: inp.ResourceType,
			Name:         inp.Name,
			DisplayName:  inp.DisplayName,
			Description:  inp.Description,
			Attributes:   inp.Attributes,
			Children:     toResourceInputs(inp.Children),
		}
	}
	return result
}

func toRoleInputs(inputs []*pb.CreateRoleInput) []accessobject.RoleInput {
	result := make([]accessobject.RoleInput, len(inputs))
	for i, inp := range inputs {
		result[i] = accessobject.RoleInput{
			ResourceNames: inp.ResourceNames,
			Name:          inp.Name,
			DisplayName:   inp.DisplayName,
			Description:   inp.Description,
			Permissions:   inp.Permissions,
			Attributes:    inp.Attributes,
			Labels:        toLabels(inp.Labels),
			Children:      toRoleInputs(inp.Children),
		}
	}
	return result
}

func toLabels(l *pb.Labels) domain.Labels {
	if l == nil || len(l.Entries) == 0 {
		return domain.Labels{}
	}
	entries := make(map[string][]string, len(l.Entries))
	for k, sl := range l.Entries {
		if sl != nil {
			entries[k] = sl.Values
		}
	}
	return domain.Labels{Entries: entries}
}

// ─── entity → proto ───

func toAccessObjectProto(ao *domain.AccessObject) *pb.AccessObject {
	p := &pb.AccessObject{
		Uid:         ao.UID,
		Environment: toEnvironmentProto(&ao.Environment),
		Lifecycle:   toLifecycleProto(&ao.Lifecycle),
	}
	for i := range ao.Resources {
		p.Resources = append(p.Resources, toResourceProto(&ao.Resources[i]))
	}
	for i := range ao.Roles {
		p.Roles = append(p.Roles, toRoleProto(&ao.Roles[i]))
	}
	return p
}

func toEnvironmentProto(e *domain.Environment) *pb.Environment {
	return &pb.Environment{
		Uid:         e.UID,
		SystemId:    e.SystemID,
		Name:        e.Name,
		DisplayName: e.DisplayName,
		Description: e.Description,
		Attributes:  e.Attributes,
	}
}

func toLifecycleProto(l *domain.Lifecycle) *pb.Lifecycle {
	p := &pb.Lifecycle{
		Status:    toLifecycleStatusProto(l.Status),
		Version:   l.Version,
		CreatedAt: timestamppb.New(l.CreatedAt),
		UpdatedAt: timestamppb.New(l.UpdatedAt),
	}
	if l.PublishedAt != nil {
		p.PublishedAt = timestamppb.New(*l.PublishedAt)
	}
	if l.RetiredAt != nil {
		p.RetiredAt = timestamppb.New(*l.RetiredAt)
	}
	return p
}

func toLifecycleStatusProto(s domain.LifecycleStatus) pb.LifecycleStatus {
	switch s {
	case domain.LifecycleStatusDraft:
		return pb.LifecycleStatus_LIFECYCLE_STATUS_DRAFT
	case domain.LifecycleStatusReview:
		return pb.LifecycleStatus_LIFECYCLE_STATUS_REVIEW
	case domain.LifecycleStatusPublished:
		return pb.LifecycleStatus_LIFECYCLE_STATUS_PUBLISHED
	case domain.LifecycleStatusRetired:
		return pb.LifecycleStatus_LIFECYCLE_STATUS_RETIRED
	default:
		return pb.LifecycleStatus_LIFECYCLE_STATUS_UNSPECIFIED
	}
}

func toResourceProto(r *domain.Resource) *pb.Resource {
	p := &pb.Resource{
		Uid:            r.UID,
		EnvironmentUid: r.AccessObjectUID,
		ParentUid:      r.ParentUID,
		ResourceType:   r.ResourceType,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		Description:    r.Description,
		Path:           r.Path,
		Attributes:     r.Attributes,
	}
	for i := range r.Children {
		p.Children = append(p.Children, toResourceProto(&r.Children[i]))
	}
	return p
}

func toRoleProto(r *domain.Role) *pb.Role {
	p := &pb.Role{
		Uid:          r.UID,
		ResourceUids: r.ResourceUIDs,
		Name:         r.Name,
		DisplayName:  r.DisplayName,
		Description:  r.Description,
		Permissions:  r.Permissions,
		Attributes:   r.Attributes,
		Labels:       toLabelsProto(r.Labels),
	}
	for i := range r.Children {
		p.Children = append(p.Children, toRoleProto(&r.Children[i]))
	}
	return p
}

func toLabelsProto(l domain.Labels) *pb.Labels {
	if len(l.Entries) == 0 {
		return &pb.Labels{}
	}
	entries := make(map[string]*pb.StringList, len(l.Entries))
	for k, vs := range l.Entries {
		entries[k] = &pb.StringList{Values: vs}
	}
	return &pb.Labels{Entries: entries}
}

// ─── pagination helpers ───

func pageFromProto(p *pb.PageRequest) (page int32, size int32) {
	if p == nil {
		return 1, 20
	}
	size = p.PageSize
	if size <= 0 {
		size = 20
	}
	return 1, size
}

// ─── error mapping ───

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "already exists")
	case errors.Is(err, domain.ErrAccessObjectDraft):
		return status.Error(codes.FailedPrecondition, "access object must be in DRAFT status")
	case errors.Is(err, domain.ErrDeleteRestricted):
		return status.Error(codes.FailedPrecondition, "can only delete access objects in DRAFT or RETIRED status")
	case errors.Is(err, domain.ErrInvalidStatus):
		return status.Error(codes.InvalidArgument, "invalid lifecycle status")
	case errors.Is(err, domain.ErrResourceNotInObject):
		return status.Error(codes.InvalidArgument, "resource does not belong to this access object")
	case errors.Is(err, domain.ErrParentNotFound):
		return status.Error(codes.NotFound, "parent resource not found")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
