package domain

import "time"

// ─── Lifecycle ───

type LifecycleStatus string

const (
	LifecycleStatusDraft     LifecycleStatus = "DRAFT"
	LifecycleStatusReview    LifecycleStatus = "REVIEW"
	LifecycleStatusPublished LifecycleStatus = "PUBLISHED"
	LifecycleStatusRetired   LifecycleStatus = "RETIRED"
)

type Lifecycle struct {
	Status      LifecycleStatus
	Version     int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedAt *time.Time
	RetiredAt   *time.Time
}

// ─── Source ───

type Source struct {
	Provider     string
	ExternalID   string
	LastSyncedAt *time.Time
}

// ─── Labels (ABAC) ───

type Labels struct {
	Entries map[string][]string
}

// ─── AccessObject ───

type AccessObject struct {
	UID         string
	Environment Environment
	Resources   []Resource
	Roles       []Role
	Lifecycle   Lifecycle
}

// ─── Environment ───

type Environment struct {
	UID         string
	SystemID    string
	Name        string
	DisplayName string
	Description string
	Attributes  map[string]string
	Source      *Source
}

// ─── Resource ───

type Resource struct {
	UID             string
	AccessObjectUID string
	ParentUID       string
	ResourceType    string
	Name            string
	DisplayName     string
	Description     string
	Path            string // ltree path
	Attributes      map[string]string
	Source          *Source
	Children        []Resource // вложенные ресурсы (дерево)
}

// ─── Role ───

type Role struct {
	UID             string
	AccessObjectUID string
	ParentRoleUID   string
	ResourceUIDs    []string
	Name            string
	DisplayName     string
	Description     string
	Permissions     []string
	Attributes      map[string]string
	Labels          Labels
	Children        []Role
	Source          *Source
}
