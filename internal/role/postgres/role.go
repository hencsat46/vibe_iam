package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"temp/internal/domain"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func (p *postgres) Add(ctx context.Context, r *domain.Role) error {
	logger := p.logger.WithMethod("Add")
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

	if err := insertRoleResources(ctx, tx, r.UID, r.ResourceUIDs); err != nil {
		logger.Error("couldn't insert role resources", zap.Error(err))
		return err
	}

	return tx.Commit(ctx)
}

func (p *postgres) GetByUID(ctx context.Context, uid string) (*domain.Role, error) {
	logger := p.logger.WithMethod("GetByUID")
	logger.Info("Entering...", zap.String("uid", uid))

	query, args, err := psql.Select(
		"uid", "access_object_uid", "COALESCE(parent_role_uid,'')", "name",
		"display_name", "description", "permissions", "attributes", "labels", "source",
	).
		From("roles").
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	r, err := scanRole(p.pgxpool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("entity not found")
			return nil, domain.ErrNotFound
		}
		logger.Error("couldn't execute sql query", zap.Error(err))
		return nil, fmt.Errorf("scan role: %w", err)
	}

	r.ResourceUIDs, err = p.getResourceUIDs(ctx, uid)
	if err != nil {
		return nil, err
	}
	r.Children, err = p.getChildren(ctx, uid)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (p *postgres) List(ctx context.Context, f domain.RoleFilter) ([]domain.Role, int32, error) {
	logger := p.logger.WithMethod("List")
	logger.Info("Entering...")

	base := psql.Select().From("roles r").Where("r.parent_role_uid IS NULL")
	if f.AccessObjectUID != "" {
		base = base.Where(sq.Eq{"r.access_object_uid": f.AccessObjectUID})
	}
	if f.ResourceUID != "" {
		base = base.Join("role_resources rr ON rr.role_uid = r.uid AND rr.resource_uid = ?", f.ResourceUID)
	}

	countQuery, countArgs, err := base.Columns("COUNT(DISTINCT r.uid)").ToSql()
	if err != nil {
		logger.Error("couldn't build count query", zap.Error(err))
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", countQuery), zap.Any("args", countArgs))

	var total int32
	if err := p.pgxpool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		logger.Error("couldn't count roles", zap.Error(err))
		return nil, 0, fmt.Errorf("count roles: %w", err)
	}

	listQuery, listArgs, err := base.
		Columns(
			"DISTINCT r.uid", "r.access_object_uid", "COALESCE(r.parent_role_uid,'')", "r.name",
			"r.display_name", "r.description", "r.permissions", "r.attributes", "r.labels", "r.source",
		).
		OrderBy("r.name").
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
		logger.Error("couldn't query roles", zap.Error(err))
		return nil, 0, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var result []domain.Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, 0, err
		}
		r.ResourceUIDs, err = p.getResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, 0, err
		}
		r.Children, err = p.getChildren(ctx, r.UID)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *r)
	}
	return result, total, rows.Err()
}

func (p *postgres) Update(ctx context.Context, uid string, upd domain.RoleUpdate) (*domain.Role, error) {
	logger := p.logger.WithMethod("Update")
	logger.Info("Entering...", zap.String("uid", uid))

	perms, err := json.Marshal(upd.Permissions)
	if err != nil {
		return nil, fmt.Errorf("marshal permissions: %w", err)
	}
	attrs, err := json.Marshal(upd.Attributes)
	if err != nil {
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}
	labels, err := json.Marshal(upd.Labels.Entries)
	if err != nil {
		return nil, fmt.Errorf("marshal labels: %w", err)
	}

	roleQuery, roleArgs, err := psql.Update("roles").
		Set("display_name", upd.DisplayName).
		Set("description", upd.Description).
		Set("permissions", perms).
		Set("attributes", attrs).
		Set("labels", labels).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", roleQuery), zap.Any("args", roleArgs))

	tx, err := p.pgxpool.Begin(ctx)
	if err != nil {
		logger.Error("couldn't begin transaction", zap.Error(err))
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, roleQuery, roleArgs...); err != nil {
		logger.Error("couldn't execute sql query", zap.Error(err))
		return nil, fmt.Errorf("update role: %w", err)
	}

	delQuery, delArgs, err := psql.Delete("role_resources").
		Where(sq.Eq{"role_uid": uid}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	if _, err := tx.Exec(ctx, delQuery, delArgs...); err != nil {
		logger.Error("couldn't clear role resources", zap.Error(err))
		return nil, fmt.Errorf("clear role resources: %w", err)
	}

	if err := insertRoleResources(ctx, tx, uid, upd.ResourceUIDs); err != nil {
		logger.Error("couldn't insert role resources", zap.Error(err))
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p.GetByUID(ctx, uid)
}

func (p *postgres) Remove(ctx context.Context, uid string) error {
	logger := p.logger.WithMethod("Remove")
	logger.Info("Entering...", zap.String("uid", uid))

	query, args, err := psql.Delete("roles").
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
		return fmt.Errorf("delete role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		logger.Warn("entity not found")
		return domain.ErrNotFound
	}
	return nil
}

// ─── helpers ───

func (p *postgres) getResourceUIDs(ctx context.Context, roleUID string) ([]string, error) {
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

func (p *postgres) getChildren(ctx context.Context, parentUID string) ([]domain.Role, error) {
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
		return nil, fmt.Errorf("query children: %w", err)
	}
	defer rows.Close()

	var result []domain.Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		r.ResourceUIDs, err = p.getResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		result = append(result, *r)
	}
	return result, rows.Err()
}

func insertRoleResources(ctx context.Context, tx pgx.Tx, roleUID string, resourceUIDs []string) error {
	for _, rUID := range resourceUIDs {
		query, args, err := psql.Insert("role_resources").
			Columns("role_uid", "resource_uid").
			Values(roleUID, rUID).
			Suffix("ON CONFLICT DO NOTHING").
			ToSql()
		if err != nil {
			return fmt.Errorf("build query: %w", err)
		}
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("insert role_resource: %w", err)
		}
	}
	return nil
}

func pageOffset(page, size int32) int32 {
	if page <= 1 {
		return 0
	}
	return (page - 1) * size
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
