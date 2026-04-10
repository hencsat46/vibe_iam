package grpc

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

// ─── domain → proto ───

func domainAOToProto(ao *domain.AccessObject) *pb.AccessObject {
	p := &pb.AccessObject{
		Uid:         ao.UID,
		Environment: domainEnvToProto(&ao.Environment),
		Lifecycle:   domainLifecycleToProto(&ao.Lifecycle),
	}
	for _, r := range ao.Resources {
		r := r
		p.Resources = append(p.Resources, domainResourceToProto(&r))
	}
	for _, r := range ao.Roles {
		r := r
		p.Roles = append(p.Roles, domainRoleToProto(&r))
	}
	return p
}

func domainEnvToProto(e *domain.Environment) *pb.Environment {
	return &pb.Environment{
		Uid:         e.UID,
		SystemId:    e.SystemID,
		Name:        e.Name,
		DisplayName: e.DisplayName,
		Description: e.Description,
		Attributes:  e.Attributes,
	}
}

func domainLifecycleToProto(l *domain.Lifecycle) *pb.Lifecycle {
	p := &pb.Lifecycle{
		Status:    domainStatusToProto(l.Status),
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

func domainStatusToProto(s domain.LifecycleStatus) pb.LifecycleStatus {
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

func domainResourceToProto(r *domain.Resource) *pb.Resource {
	fmt.Printf("[DEBUG] domainResourceToProto: name=%s children=%d\n", r.Name, len(r.Children))
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
	for _, c := range r.Children {
		c := c
		p.Children = append(p.Children, domainResourceToProto(&c))
	}
	return p
}

func domainRoleToProto(r *domain.Role) *pb.Role {
	p := &pb.Role{
		Uid:          r.UID,
		ResourceUids: r.ResourceUIDs,
		Name:         r.Name,
		DisplayName:  r.DisplayName,
		Description:  r.Description,
		Permissions:  r.Permissions,
		Attributes:   r.Attributes,
		Labels:       domainLabelsToProto(r.Labels),
	}
	for _, c := range r.Children {
		c := c
		p.Children = append(p.Children, domainRoleToProto(&c))
	}
	return p
}

func domainLabelsToProto(l domain.Labels) *pb.Labels {
	if len(l.Entries) == 0 {
		return &pb.Labels{}
	}
	entries := make(map[string]*pb.StringList, len(l.Entries))
	for k, vs := range l.Entries {
		entries[k] = &pb.StringList{Values: vs}
	}
	return &pb.Labels{Entries: entries}
}

// ─── proto → domain ───

func protoLabelsToDomain(l *pb.Labels) domain.Labels {
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

// ─── error mapping ───

func domainErrToGRPC(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrAccessObjectDraft):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrDeleteRestricted):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrInvalidStatus):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrResourceNotInObject):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrParentNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// ─── pagination helpers ───

func pageRequestFromProto(p *pb.PageRequest) (page int32, size int32) {
	if p == nil {
		return 1, 20
	}
	size = p.PageSize
	if size <= 0 {
		size = 20
	}
	// Simple numeric page token
	page = 1
	return page, size
}
