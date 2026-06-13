package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"temp/internal/domain"
)

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

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func (p *postgres) Create(ctx context.Context, ao *domain.AccessObject) error {
	logger := p.logger.WithMethod("Create")
	logger.Info("Entering...")

	attrs, err := json.Marshal(ao.Environment.Attributes)
	if err != nil {
		logger.Error("couldn't marshal attributes", zap.Error(err))
		return fmt.Errorf("marshal attributes: %w", err)
	}

	query, args, err := psql.Insert("access_objects").
		Columns("uid", "env_uid", "system_id", "env_name", "display_name", "description", "attributes", "status", "version").
		Values(
			ao.UID,
			ao.Environment.UID,
			ao.Environment.SystemID,
			ao.Environment.Name,
			ao.Environment.DisplayName,
			ao.Environment.Description,
			attrs,
			string(ao.Lifecycle.Status),
			ao.Lifecycle.Version,
		).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	if _, err = p.pgxpool.Exec(ctx, query, args...); err != nil {
		if strings.Contains(err.Error(), "unique") {
			return domain.ErrAlreadyExists
		}
		logger.Error("couldn't execute sql query", zap.Error(err))
		return fmt.Errorf("insert access object: %w", err)
	}
	return nil
}

func (p *postgres) GetByUID(ctx context.Context, uid string) (*domain.AccessObject, error) {
	logger := p.logger.WithMethod("GetByUID")
	logger.Info("Entering...", zap.String("uid", uid))

	query, args, err := psql.Select(
		"uid", "env_uid", "system_id", "env_name", "display_name", "description",
		"attributes", "source", "status", "version",
		"created_at", "updated_at", "published_at", "retired_at",
	).
		From("access_objects").
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	r := &aoRow{}
	err = p.pgxpool.QueryRow(ctx, query, args...).Scan(
		&r.UID, &r.EnvUID, &r.SystemID, &r.EnvName, &r.DisplayName, &r.Description,
		&r.Attributes, &r.Source, &r.Status, &r.Version,
		&r.CreatedAt, &r.UpdatedAt, &r.PublishedAt, &r.RetiredAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("entity not found")
			return nil, domain.ErrNotFound
		}
		logger.Error("couldn't execute sql query", zap.Error(err))
		return nil, fmt.Errorf("scan access object: %w", err)
	}

	ao, err := r.toDomain()
	if err != nil {
		return nil, err
	}
	ao.Resources, err = p.loadResources(ctx, uid)
	if err != nil {
		return nil, err
	}
	ao.Roles, err = p.loadRoles(ctx, uid)
	if err != nil {
		return nil, err
	}
	return ao, nil
}

func (p *postgres) loadResources(ctx context.Context, aoUID string) ([]domain.Resource, error) {
	query, args, err := psql.Select(
		"uid", "access_object_uid", "COALESCE(parent_uid,'')", "resource_type", "name",
		"display_name", "description", "path::text", "attributes", "source",
	).
		From("resources").
		Where(sq.Eq{"access_object_uid": aoUID}).
		OrderBy("path").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := p.pgxpool.Query(ctx, query, args...)
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
	return buildResourceTree(flat), nil
}

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

func (p *postgres) loadRoles(ctx context.Context, aoUID string) ([]domain.Role, error) {
	query, args, err := psql.Select(
		"uid", "access_object_uid", "COALESCE(parent_role_uid,'')", "name",
		"display_name", "description", "permissions", "attributes", "labels", "source",
	).
		From("roles").
		Where(sq.Eq{"access_object_uid": aoUID, "parent_role_uid": nil}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := p.pgxpool.Query(ctx, query, args...)
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
		r.Children, err = p.loadChildRoles(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		r.ResourceUIDs, err = p.loadRoleResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *r)
	}
	return roles, rows.Err()
}

func (p *postgres) loadChildRoles(ctx context.Context, parentUID string) ([]domain.Role, error) {
	query, args, err := psql.Select(
		"uid", "access_object_uid", "COALESCE(parent_role_uid,'')", "name",
		"display_name", "description", "permissions", "attributes", "labels", "source",
	).
		From("roles").
		Where(sq.Eq{"parent_role_uid": parentUID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := p.pgxpool.Query(ctx, query, args...)
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
		rUIDs, err := p.loadRoleResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		r.ResourceUIDs = rUIDs
		r.Children, err = p.loadChildRoles(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *r)
	}
	return roles, rows.Err()
}

func (p *postgres) loadRoleResourceUIDs(ctx context.Context, roleUID string) ([]string, error) {
	query, args, err := psql.Select("resource_uid").
		From("role_resources").
		Where(sq.Eq{"role_uid": roleUID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := p.pgxpool.Query(ctx, query, args...)
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

func (p *postgres) List(ctx context.Context, f domain.AccessObjectFilter) ([]domain.AccessObject, int32, error) {
	logger := p.logger.WithMethod("List")
	logger.Info("Entering...")

	base := psql.Select().From("access_objects")
	if f.SystemID != "" {
		base = base.Where(sq.Eq{"system_id": f.SystemID})
	}
	if f.Status != "" {
		base = base.Where(sq.Eq{"status": f.Status})
	}

	countQuery, countArgs, err := base.Columns("COUNT(*)").ToSql()
	if err != nil {
		logger.Error("couldn't build count query", zap.Error(err))
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", countQuery), zap.Any("args", countArgs))

	var total int32
	if err := p.pgxpool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		logger.Error("couldn't count access objects", zap.Error(err))
		return nil, 0, fmt.Errorf("count access objects: %w", err)
	}

	listQuery, listArgs, err := base.
		Columns(
			"uid", "env_uid", "system_id", "env_name", "display_name", "description",
			"attributes", "source", "status", "version",
			"created_at", "updated_at", "published_at", "retired_at",
		).
		OrderBy("created_at DESC").
		Limit(uint64(f.PageSize)).
		Offset(uint64(pageOffset(f.Page, f.PageSize))).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, 0, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", listQuery), zap.Any("args", listArgs))

	rows, err := p.pgxpool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		logger.Error("couldn't query access objects", zap.Error(err))
		return nil, 0, fmt.Errorf("list access objects: %w", err)
	}
	defer rows.Close()

	var result []domain.AccessObject
	for rows.Next() {
		r := &aoRow{}
		if err := rows.Scan(
			&r.UID, &r.EnvUID, &r.SystemID, &r.EnvName, &r.DisplayName, &r.Description,
			&r.Attributes, &r.Source, &r.Status, &r.Version,
			&r.CreatedAt, &r.UpdatedAt, &r.PublishedAt, &r.RetiredAt,
		); err != nil {
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

func (p *postgres) UpdateEnvironment(ctx context.Context, uid string, upd domain.EnvironmentUpdate) (*domain.AccessObject, error) {
	logger := p.logger.WithMethod("UpdateEnvironment")
	logger.Info("Entering...", zap.String("uid", uid))

	attrs, err := json.Marshal(upd.Attributes)
	if err != nil {
		logger.Error("couldn't marshal attributes", zap.Error(err))
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}

	query, args, err := psql.Update("access_objects").
		Set("display_name", upd.DisplayName).
		Set("description", upd.Description).
		Set("attributes", attrs).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	if _, err = p.pgxpool.Exec(ctx, query, args...); err != nil {
		logger.Error("couldn't execute sql query", zap.Error(err))
		return nil, fmt.Errorf("update environment: %w", err)
	}
	return p.GetByUID(ctx, uid)
}

func (p *postgres) Delete(ctx context.Context, uid string) error {
	logger := p.logger.WithMethod("Delete")
	logger.Info("Entering...", zap.String("uid", uid))

	query, args, err := psql.Delete("access_objects").
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	tag, err := p.pgxpool.Exec(ctx, query, args...)
	if err != nil {
		logger.Error("couldn't execute sql query", zap.Error(err))
		return fmt.Errorf("delete access object: %w", err)
	}
	if tag.RowsAffected() == 0 {
		logger.Warn("entity not found")
		return domain.ErrNotFound
	}
	return nil
}

func (p *postgres) Search(ctx context.Context, q domain.SearchQuery) ([]domain.AccessObject, int32, error) {
	logger := p.logger.WithMethod("Search")
	logger.Info("Entering...")

	base := psql.Select().From("access_objects ao")

	if q.ResourceType != "" {
		base = base.Join("resources r ON r.access_object_uid = ao.uid AND r.resource_type = ?", q.ResourceType)
	}
	if q.Query != "" {
		like := "%" + strings.ToLower(q.Query) + "%"
		base = base.Where(
			sq.Or{
				sq.Expr("LOWER(ao.env_name) LIKE ?", like),
				sq.Expr("LOWER(ao.display_name) LIKE ?", like),
				sq.Expr("LOWER(ao.system_id) LIKE ?", like),
			},
		)
	}
	if q.SystemID != "" {
		base = base.Where(sq.Eq{"ao.system_id": q.SystemID})
	}
	if q.Status != "" {
		base = base.Where(sq.Eq{"ao.status": q.Status})
	}

	countQuery, countArgs, err := base.Columns("COUNT(DISTINCT ao.uid)").ToSql()
	if err != nil {
		logger.Error("couldn't build count query", zap.Error(err))
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", countQuery), zap.Any("args", countArgs))

	var total int32
	if err := p.pgxpool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		logger.Error("couldn't count search results", zap.Error(err))
		return nil, 0, fmt.Errorf("count search: %w", err)
	}

	listQuery, listArgs, err := base.
		Columns(
			"DISTINCT ao.uid", "ao.env_uid", "ao.system_id", "ao.env_name", "ao.display_name", "ao.description",
			"ao.attributes", "ao.source", "ao.status", "ao.version",
			"ao.created_at", "ao.updated_at", "ao.published_at", "ao.retired_at",
		).
		OrderBy("ao.created_at DESC").
		Limit(uint64(q.PageSize)).
		Offset(uint64(pageOffset(q.Page, q.PageSize))).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, 0, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", listQuery), zap.Any("args", listArgs))

	rows, err := p.pgxpool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		logger.Error("couldn't execute search query", zap.Error(err))
		return nil, 0, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var result []domain.AccessObject
	for rows.Next() {
		r := &aoRow{}
		if err := rows.Scan(
			&r.UID, &r.EnvUID, &r.SystemID, &r.EnvName, &r.DisplayName, &r.Description,
			&r.Attributes, &r.Source, &r.Status, &r.Version,
			&r.CreatedAt, &r.UpdatedAt, &r.PublishedAt, &r.RetiredAt,
		); err != nil {
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

// ─── resource operations (for CreateAccessObject) ───

func (p *postgres) AddResource(ctx context.Context, r *domain.Resource) error {
	logger := p.logger.WithMethod("AddResource")
	logger.Info("Entering...", zap.String("name", r.Name))

	attrs, err := json.Marshal(r.Attributes)
	if err != nil {
		logger.Error("couldn't marshal attributes", zap.Error(err))
		return fmt.Errorf("marshal attributes: %w", err)
	}

	var parentUID *string
	if r.ParentUID != "" {
		parentUID = &r.ParentUID
	}

	query, args, err := psql.Insert("resources").
		Columns("uid", "access_object_uid", "parent_uid", "resource_type", "name", "display_name", "description", "path", "attributes").
		Values(r.UID, r.AccessObjectUID, parentUID, r.ResourceType, r.Name, r.DisplayName, r.Description, sq.Expr("?::ltree", r.Path), attrs).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	if _, err = p.pgxpool.Exec(ctx, query, args...); err != nil {
		logger.Error("couldn't execute sql query", zap.Error(err))
		return fmt.Errorf("insert resource: %w", err)
	}
	return nil
}

func (p *postgres) GetResourcePath(ctx context.Context, uid string) (string, error) {
	logger := p.logger.WithMethod("GetResourcePath")
	logger.Info("Entering...", zap.String("uid", uid))

	query, args, err := psql.Select("path::text").
		From("resources").
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return "", fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	var path string
	if err := p.pgxpool.QueryRow(ctx, query, args...).Scan(&path); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("resource not found")
			return "", domain.ErrNotFound
		}
		logger.Error("couldn't execute sql query", zap.Error(err))
		return "", err
	}
	return path, nil
}

// ─── role operations (for CreateAccessObject) ───

func (p *postgres) AddRole(ctx context.Context, r *domain.Role) error {
	logger := p.logger.WithMethod("AddRole")
	logger.Info("Entering...", zap.String("name", r.Name))

	perms, err := json.Marshal(r.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}
	attrs, err := json.Marshal(r.Attributes)
	if err != nil {
		return fmt.Errorf("marshal attributes: %w", err)
	}
	labels, err := json.Marshal(r.Labels.Entries)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}

	var parentUID *string
	if r.ParentRoleUID != "" {
		parentUID = &r.ParentRoleUID
	}

	roleQuery, roleArgs, err := psql.Insert("roles").
		Columns("uid", "access_object_uid", "parent_role_uid", "name", "display_name", "description", "permissions", "attributes", "labels").
		Values(r.UID, r.AccessObjectUID, parentUID, r.Name, r.DisplayName, r.Description, perms, attrs, labels).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", roleQuery), zap.Any("args", roleArgs))

	tx, err := p.pgxpool.Begin(ctx)
	if err != nil {
		logger.Error("couldn't begin transaction", zap.Error(err))
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, roleQuery, roleArgs...); err != nil {
		logger.Error("couldn't execute sql query", zap.Error(err))
		return fmt.Errorf("insert role: %w", err)
	}

	for _, rUID := range r.ResourceUIDs {
		rrQuery, rrArgs, err := psql.Insert("role_resources").
			Columns("role_uid", "resource_uid").
			Values(r.UID, rUID).
			Suffix("ON CONFLICT DO NOTHING").
			ToSql()
		if err != nil {
			return fmt.Errorf("build role_resource query: %w", err)
		}
		if _, err := tx.Exec(ctx, rrQuery, rrArgs...); err != nil {
			logger.Error("couldn't insert role_resource", zap.Error(err))
			return fmt.Errorf("insert role_resource: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ─── helpers ───

func pageOffset(page, size int32) int32 {
	if page <= 1 {
		return 0
	}
	return (page - 1) * size
}

func scanResource(s interface{ Scan(...any) error }) (*domain.Resource, error) {
	var attrsB, srcB []byte
	r := &domain.Resource{}

	if err := s.Scan(
		&r.UID, &r.AccessObjectUID, &r.ParentUID, &r.ResourceType, &r.Name,
		&r.DisplayName, &r.Description, &r.Path, &attrsB, &srcB,
	); err != nil {
		return nil, err
	}

	if len(attrsB) > 0 {
		if err := json.Unmarshal(attrsB, &r.Attributes); err != nil {
			return nil, fmt.Errorf("unmarshal resource attributes: %w", err)
		}
	}
	if r.Attributes == nil {
		r.Attributes = map[string]string{}
	}
	return r, nil
}

func scanRole(s interface{ Scan(...any) error }) (*domain.Role, error) {
	var permsB, attrsB, labelsB, srcB []byte
	r := &domain.Role{}

	if err := s.Scan(
		&r.UID, &r.AccessObjectUID, &r.ParentRoleUID, &r.Name,
		&r.DisplayName, &r.Description, &permsB, &attrsB, &labelsB, &srcB,
	); err != nil {
		return nil, err
	}

	if len(permsB) > 0 {
		if err := json.Unmarshal(permsB, &r.Permissions); err != nil {
			return nil, fmt.Errorf("unmarshal permissions: %w", err)
		}
	}
	if len(attrsB) > 0 {
		if err := json.Unmarshal(attrsB, &r.Attributes); err != nil {
			return nil, fmt.Errorf("unmarshal attributes: %w", err)
		}
	}
	if len(labelsB) > 0 {
		entries := map[string][]string{}
		if err := json.Unmarshal(labelsB, &entries); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
		r.Labels = domain.Labels{Entries: entries}
	}

	if r.Attributes == nil {
		r.Attributes = map[string]string{}
	}
	if r.Permissions == nil {
		r.Permissions = []string{}
	}
	return r, nil
}
