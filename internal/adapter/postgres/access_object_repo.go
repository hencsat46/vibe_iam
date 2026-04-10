package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"temp/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccessObjectRepo struct {
	db *pgxpool.Pool
}

func NewAccessObjectRepo(db *pgxpool.Pool) *AccessObjectRepo {
	return &AccessObjectRepo{db: db}
}

// ─── row scanning helpers ───

type aoRow struct {
	UID         string
	EnvUID      string
	SystemID    string
	EnvName     string
	DisplayName string
	Description string
	Attributes  []byte
	Source      []byte
	Status      string
	Version     int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedAt *time.Time
	RetiredAt   *time.Time
}

func (r *aoRow) toDomain() (*domain.AccessObject, error) {
	attrs := map[string]string{}
	if len(r.Attributes) > 0 {
		if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
			return nil, fmt.Errorf("unmarshal attributes: %w", err)
		}
	}

	var src *domain.Source
	if len(r.Source) > 0 && string(r.Source) != "null" {
		src = &domain.Source{}
		if err := json.Unmarshal(r.Source, src); err != nil {
			return nil, fmt.Errorf("unmarshal source: %w", err)
		}
	}

	return &domain.AccessObject{
		UID: r.UID,
		Environment: domain.Environment{
			UID:         r.EnvUID,
			SystemID:    r.SystemID,
			Name:        r.EnvName,
			DisplayName: r.DisplayName,
			Description: r.Description,
			Attributes:  attrs,
			Source:      src,
		},
		Lifecycle: domain.Lifecycle{
			Status:      domain.LifecycleStatus(r.Status),
			Version:     r.Version,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			PublishedAt: r.PublishedAt,
			RetiredAt:   r.RetiredAt,
		},
	}, nil
}

func (repo *AccessObjectRepo) Create(ctx context.Context, ao *domain.AccessObject) error {
	attrs, err := json.Marshal(ao.Environment.Attributes)
	if err != nil {
		return fmt.Errorf("marshal attributes: %w", err)
	}

	_, err = repo.db.Exec(ctx, `
		INSERT INTO access_objects
			(uid, env_uid, system_id, env_name, display_name, description, attributes, status, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		ao.UID,
		ao.Environment.UID,
		ao.Environment.SystemID,
		ao.Environment.Name,
		ao.Environment.DisplayName,
		ao.Environment.Description,
		attrs,
		string(ao.Lifecycle.Status),
		ao.Lifecycle.Version,
	)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("%w: system_id=%s env=%s", domain.ErrAlreadyExists, ao.Environment.SystemID, ao.Environment.Name)
		}
		return fmt.Errorf("insert access object: %w", err)
	}
	return nil
}

func (repo *AccessObjectRepo) GetByUID(ctx context.Context, uid string) (*domain.AccessObject, error) {
	row := repo.db.QueryRow(ctx, `
		SELECT uid, env_uid, system_id, env_name, display_name, description,
		       attributes, source, status, version,
		       created_at, updated_at, published_at, retired_at
		FROM access_objects
		WHERE uid = $1`, uid)

	r := &aoRow{}
	err := row.Scan(
		&r.UID, &r.EnvUID, &r.SystemID, &r.EnvName, &r.DisplayName, &r.Description,
		&r.Attributes, &r.Source, &r.Status, &r.Version,
		&r.CreatedAt, &r.UpdatedAt, &r.PublishedAt, &r.RetiredAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("access object %s: %w", uid, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("scan access object: %w", err)
	}

	ao, err := r.toDomain()
	if err != nil {
		return nil, err
	}

	ao.Resources, err = repo.loadResources(ctx, uid)
	if err != nil {
		return nil, err
	}
	ao.Roles, err = repo.loadRoles(ctx, uid)
	if err != nil {
		return nil, err
	}
	return ao, nil
}

func (repo *AccessObjectRepo) loadResources(ctx context.Context, aoUID string) ([]domain.Resource, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT uid, access_object_uid, COALESCE(parent_uid,''), resource_type, name,
		       display_name, description, path::text, attributes, source
		FROM resources
		WHERE access_object_uid = $1
		ORDER BY path`, aoUID)
	if err != nil {
		return nil, fmt.Errorf("query resources: %w", err)
	}
	defer rows.Close()

	var flat []domain.Resource
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		flat = append(flat, *r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tree := buildResourceTree(flat)
	fmt.Printf("[DEBUG] loadResources: flat=%d, roots=%d\n", len(flat), len(tree))
	return tree, nil
}

// buildResourceTree converts a flat (path-ordered) slice into a nested tree.
func buildResourceTree(flat []domain.Resource) []domain.Resource {
	type node struct {
		res      domain.Resource
		children []*node
	}

	nodes := make(map[string]*node, len(flat))
	for _, r := range flat {
		r := r
		nodes[r.UID] = &node{res: r}
	}

	var roots []*node
	for _, r := range flat {
		n := nodes[r.UID]
		if r.ParentUID == "" {
			roots = append(roots, n)
		} else if parent, ok := nodes[r.ParentUID]; ok {
			parent.children = append(parent.children, n)
		}
	}

	var toResource func(*node) domain.Resource
	toResource = func(n *node) domain.Resource {
		res := n.res
		res.Children = make([]domain.Resource, len(n.children))
		for i, c := range n.children {
			res.Children[i] = toResource(c)
		}
		return res
	}

	result := make([]domain.Resource, len(roots))
	for i, root := range roots {
		result[i] = toResource(root)
	}
	return result
}

func (repo *AccessObjectRepo) loadRoles(ctx context.Context, aoUID string) ([]domain.Role, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT r.uid, r.access_object_uid, COALESCE(r.parent_role_uid,''), r.name,
		       r.display_name, r.description, r.permissions, r.attributes, r.labels, r.source
		FROM roles r
		WHERE r.access_object_uid = $1 AND r.parent_role_uid IS NULL`, aoUID)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		children, err := repo.loadChildRoles(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		r.Children = children

		resourceUIDs, err := repo.loadRoleResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		r.ResourceUIDs = resourceUIDs
		roles = append(roles, *r)
	}
	return roles, rows.Err()
}

func (repo *AccessObjectRepo) loadChildRoles(ctx context.Context, parentUID string) ([]domain.Role, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT uid, access_object_uid, COALESCE(parent_role_uid,''), name,
		       display_name, description, permissions, attributes, labels, source
		FROM roles
		WHERE parent_role_uid = $1`, parentUID)
	if err != nil {
		return nil, fmt.Errorf("query child roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		rUIDs, err := repo.loadRoleResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		r.ResourceUIDs = rUIDs
		roles = append(roles, *r)
	}
	return roles, rows.Err()
}

func (repo *AccessObjectRepo) loadRoleResourceUIDs(ctx context.Context, roleUID string) ([]string, error) {
	rows, err := repo.db.Query(ctx, `SELECT resource_uid FROM role_resources WHERE role_uid = $1`, roleUID)
	if err != nil {
		return nil, fmt.Errorf("query role resources: %w", err)
	}
	defer rows.Close()

	var uids []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		uids = append(uids, uid)
	}
	return uids, rows.Err()
}

func (repo *AccessObjectRepo) List(ctx context.Context, f domain.AccessObjectFilter) ([]domain.AccessObject, int32, error) {
	where, args := buildAOFilter(f)
	offset := pageOffset(f.Page, f.PageSize)

	var total int32
	err := repo.db.QueryRow(ctx, `SELECT COUNT(*) FROM access_objects`+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count access objects: %w", err)
	}

	args = append(args, f.PageSize, offset)
	rows, err := repo.db.Query(ctx, `
		SELECT uid, env_uid, system_id, env_name, display_name, description,
		       attributes, source, status, version,
		       created_at, updated_at, published_at, retired_at
		FROM access_objects`+where+
		fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list access objects: %w", err)
	}
	defer rows.Close()

	var result []domain.AccessObject
	for rows.Next() {
		r := &aoRow{}
		err := rows.Scan(
			&r.UID, &r.EnvUID, &r.SystemID, &r.EnvName, &r.DisplayName, &r.Description,
			&r.Attributes, &r.Source, &r.Status, &r.Version,
			&r.CreatedAt, &r.UpdatedAt, &r.PublishedAt, &r.RetiredAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		ao, err := r.toDomain()
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *ao)
	}
	return result, total, rows.Err()
}

func (repo *AccessObjectRepo) UpdateEnvironment(ctx context.Context, uid string, upd domain.EnvironmentUpdate) (*domain.AccessObject, error) {
	attrs, err := json.Marshal(upd.Attributes)
	if err != nil {
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}

	_, err = repo.db.Exec(ctx, `
		UPDATE access_objects
		SET display_name = $2, description = $3, attributes = $4, updated_at = NOW()
		WHERE uid = $1`,
		uid, upd.DisplayName, upd.Description, attrs,
	)
	if err != nil {
		return nil, fmt.Errorf("update environment: %w", err)
	}

	return repo.GetByUID(ctx, uid)
}

func (repo *AccessObjectRepo) Delete(ctx context.Context, uid string) error {
	tag, err := repo.db.Exec(ctx, `DELETE FROM access_objects WHERE uid = $1`, uid)
	if err != nil {
		return fmt.Errorf("delete access object: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("access object %s: %w", uid, domain.ErrNotFound)
	}
	return nil
}

func (repo *AccessObjectRepo) Search(ctx context.Context, q domain.SearchQuery) ([]domain.AccessObject, int32, error) {
	var conditions []string
	var args []any

	if q.Query != "" {
		args = append(args, "%"+strings.ToLower(q.Query)+"%")
		n := len(args)
		conditions = append(conditions, fmt.Sprintf(
			`(LOWER(ao.env_name) LIKE $%d OR LOWER(ao.display_name) LIKE $%d OR LOWER(ao.system_id) LIKE $%d)`,
			n, n, n,
		))
	}
	if q.SystemID != "" {
		args = append(args, q.SystemID)
		conditions = append(conditions, fmt.Sprintf(`ao.system_id = $%d`, len(args)))
	}
	if q.Status != "" {
		args = append(args, q.Status)
		conditions = append(conditions, fmt.Sprintf(`ao.status = $%d`, len(args)))
	}

	var where string
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	if q.ResourceType != "" {
		args = append(args, q.ResourceType)
		joinClause := fmt.Sprintf(
			` INNER JOIN resources r ON r.access_object_uid = ao.uid AND r.resource_type = $%d`, len(args))
		where = joinClause + where
	}

	offset := pageOffset(q.Page, q.PageSize)

	var total int32
	err := repo.db.QueryRow(ctx, `SELECT COUNT(DISTINCT ao.uid) FROM access_objects ao`+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count search: %w", err)
	}

	args = append(args, q.PageSize, offset)
	sql := `SELECT DISTINCT ao.uid, ao.env_uid, ao.system_id, ao.env_name, ao.display_name, ao.description,
		       ao.attributes, ao.source, ao.status, ao.version,
		       ao.created_at, ao.updated_at, ao.published_at, ao.retired_at
		FROM access_objects ao` + where +
		fmt.Sprintf(` ORDER BY ao.created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := repo.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var result []domain.AccessObject
	for rows.Next() {
		r := &aoRow{}
		err := rows.Scan(
			&r.UID, &r.EnvUID, &r.SystemID, &r.EnvName, &r.DisplayName, &r.Description,
			&r.Attributes, &r.Source, &r.Status, &r.Version,
			&r.CreatedAt, &r.UpdatedAt, &r.PublishedAt, &r.RetiredAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		ao, err := r.toDomain()
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *ao)
	}
	return result, total, rows.Err()
}

// ─── helpers ───

func buildAOFilter(f domain.AccessObjectFilter) (string, []any) {
	var conds []string
	var args []any

	if f.SystemID != "" {
		args = append(args, f.SystemID)
		conds = append(conds, fmt.Sprintf(`system_id = $%d`, len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf(`status = $%d`, len(args)))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func pageOffset(page, size int32) int32 {
	if page <= 1 {
		return 0
	}
	return (page - 1) * size
}
