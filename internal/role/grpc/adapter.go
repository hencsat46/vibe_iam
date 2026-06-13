package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

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

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "already exists")
	case errors.Is(err, domain.ErrAccessObjectDraft):
		return status.Error(codes.FailedPrecondition, "access object must be in DRAFT status")
	case errors.Is(err, domain.ErrResourceNotInObject):
		return status.Error(codes.InvalidArgument, "resource does not belong to this access object")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
