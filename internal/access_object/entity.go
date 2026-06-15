package access_object

import "temp/internal/domain"

type ResourceInput struct {
	ResourceType string
	Name         string
	DisplayName  string
	Description  string
	Attributes   map[string]string
	Children     []ResourceInput
}

type RoleInput struct {
	ResourceNames []string
	Name          string
	DisplayName   string
	Description   string
	Permissions   []string
	Attributes    map[string]string
	Labels        domain.Labels
	Children      []RoleInput
}

type CreateRequest struct {
	SystemID        string
	EnvironmentName string
	DisplayName     string
	Description     string
	Attributes      map[string]string
	Resources       []ResourceInput
	Roles           []RoleInput
}
