package domain

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
