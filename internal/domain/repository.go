package domain

import "context"

// AccessObjectRepository manages the top-level access object records.
// Resources and roles are handled by their own repositories.
type AccessObjectRepository interface {
	Create(ctx context.Context, ao *AccessObject) error
	GetByUID(ctx context.Context, uid string) (*AccessObject, error)
	List(ctx context.Context, f AccessObjectFilter) ([]AccessObject, int32, error)
	UpdateEnvironment(ctx context.Context, uid string, upd EnvironmentUpdate) (*AccessObject, error)
	Delete(ctx context.Context, uid string) error
	Search(ctx context.Context, q SearchQuery) ([]AccessObject, int32, error)
}

type ResourceRepository interface {
	Add(ctx context.Context, r *Resource) error
	GetByUID(ctx context.Context, uid string) (*Resource, error)
	List(ctx context.Context, f ResourceFilter) ([]Resource, int32, error)
	Update(ctx context.Context, uid string, upd ResourceUpdate) (*Resource, error)
	Remove(ctx context.Context, uid string) error
	GetSubtree(ctx context.Context, uid string, maxDepth int32) (*Resource, []Resource, error)
	GetPath(ctx context.Context, uid string) (string, error)
}

type RoleRepository interface {
	Add(ctx context.Context, r *Role) error
	GetByUID(ctx context.Context, uid string) (*Role, error)
	List(ctx context.Context, f RoleFilter) ([]Role, int32, error)
	Update(ctx context.Context, uid string, upd RoleUpdate) (*Role, error)
	Remove(ctx context.Context, uid string) error
}

// ─── Filters ───

type AccessObjectFilter struct {
	SystemID string
	Status   string
	Page     int32
	PageSize int32
}

type ResourceFilter struct {
	AccessObjectUID string
	ParentUID       string
	ResourceType    string
	Page            int32
	PageSize        int32
}

type RoleFilter struct {
	AccessObjectUID string
	ResourceUID     string
	Page            int32
	PageSize        int32
}

type SearchQuery struct {
	Query        string
	SystemID     string
	ResourceType string
	Status       string
	Page         int32
	PageSize     int32
}

// ─── Partial update structs ───

type EnvironmentUpdate struct {
	DisplayName string
	Description string
	Attributes  map[string]string
}

type ResourceUpdate struct {
	DisplayName string
	Description string
	Attributes  map[string]string
}

type RoleUpdate struct {
	ResourceUIDs []string
	DisplayName  string
	Description  string
	Permissions  []string
	Attributes   map[string]string
	Labels       Labels
}
