package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "temp/gen/pb/v1"
	"temp/internal/domain"
)

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
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
