package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"temp/internal/domain"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func (p *postgres) Add(ctx context.Context, r *domain.Resource) error {
	logger := p.logger.WithMethod("Add")
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

func (p *postgres) GetByUID(ctx context.Context, uid string) (*domain.Resource, error) {
	logger := p.logger.WithMethod("GetByUID")
	logger.Info("Entering...", zap.String("uid", uid))

	query, args, err := psql.Select(
		"uid", "access_object_uid", "COALESCE(parent_uid,'')", "resource_type", "name",
		"display_name", "description", "path::text", "attributes", "source",
	).
		From("resources").
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	r, err := scanResource(p.pgxpool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("entity not found")
			return nil, domain.ErrNotFound
		}
		logger.Error("couldn't execute sql query", zap.Error(err))
		return nil, fmt.Errorf("scan resource: %w", err)
	}
	return r, nil
}

func (p *postgres) List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, int32, error) {
	logger := p.logger.WithMethod("List")
	logger.Info("Entering...")

	base := psql.Select().From("resources")
	if f.AccessObjectUID != "" {
		base = base.Where(sq.Eq{"access_object_uid": f.AccessObjectUID})
	}
	if f.ParentUID != "" {
		base = base.Where(sq.Eq{"parent_uid": f.ParentUID})
	} else if f.AccessObjectUID != "" {
		base = base.Where(sq.Eq{"parent_uid": nil})
	}
	if f.ResourceType != "" {
		base = base.Where(sq.Eq{"resource_type": f.ResourceType})
	}

	countQuery, countArgs, err := base.Columns("COUNT(*)").ToSql()
	if err != nil {
		logger.Error("couldn't build count query", zap.Error(err))
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", countQuery), zap.Any("args", countArgs))

	var total int32
	if err := p.pgxpool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		logger.Error("couldn't count resources", zap.Error(err))
		return nil, 0, fmt.Errorf("count resources: %w", err)
	}

	listQuery, listArgs, err := base.
		Columns(
			"uid", "access_object_uid", "COALESCE(parent_uid,'')", "resource_type", "name",
			"display_name", "description", "path::text", "attributes", "source",
		).
		OrderBy("path").
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
		logger.Error("couldn't query resources", zap.Error(err))
		return nil, 0, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	var result []domain.Resource
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *r)
	}
	return result, total, rows.Err()
}

func (p *postgres) Update(ctx context.Context, uid string, upd domain.ResourceUpdate) (*domain.Resource, error) {
	logger := p.logger.WithMethod("Update")
	logger.Info("Entering...", zap.String("uid", uid))

	attrs, err := json.Marshal(upd.Attributes)
	if err != nil {
		logger.Error("couldn't marshal attributes", zap.Error(err))
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}

	query, args, err := psql.Update("resources").
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
		return nil, fmt.Errorf("update resource: %w", err)
	}
	return p.GetByUID(ctx, uid)
}

func (p *postgres) Remove(ctx context.Context, uid string) error {
	logger := p.logger.WithMethod("Remove")
	logger.Info("Entering...", zap.String("uid", uid))

	pathQuery, pathArgs, err := psql.Select("path::text").
		From("resources").
		Where(sq.Eq{"uid": uid}).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", pathQuery), zap.Any("args", pathArgs))

	var path string
	if err := p.pgxpool.QueryRow(ctx, pathQuery, pathArgs...).Scan(&path); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warn("entity not found")
			return domain.ErrNotFound
		}
		logger.Error("couldn't execute sql query", zap.Error(err))
		return err
	}

	delQuery, delArgs, err := psql.Delete("resources").
		Where(sq.Expr("path <@ ?::ltree", path)).
		ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", delQuery), zap.Any("args", delArgs))

	if _, err := p.pgxpool.Exec(ctx, delQuery, delArgs...); err != nil {
		logger.Error("couldn't delete resource subtree", zap.Error(err))
		return fmt.Errorf("remove resource subtree: %w", err)
	}
	return nil
}

func (p *postgres) GetSubtree(ctx context.Context, uid string, maxDepth int32) (*domain.Resource, []domain.Resource, error) {
	logger := p.logger.WithMethod("GetSubtree")
	logger.Info("Entering...", zap.String("uid", uid))

	root, err := p.GetByUID(ctx, uid)
	if err != nil {
		logger.Error("couldn't get root resource", zap.Error(err))
		return nil, nil, err
	}

	builder := psql.Select(
		"uid", "access_object_uid", "COALESCE(parent_uid,'')", "resource_type", "name",
		"display_name", "description", "path::text", "attributes", "source",
	).
		From("resources").
		Where(sq.Expr("path <@ ?::ltree", root.Path)).
		Where(sq.NotEq{"uid": uid})

	if maxDepth > 0 {
		builder = builder.Where(sq.Expr("nlevel(path) <= ?", nlevel(root.Path)+int(maxDepth)))
	}

	query, args, err := builder.OrderBy("path").ToSql()
	if err != nil {
		logger.Error("couldn't build sql query", zap.Error(err))
		return nil, nil, fmt.Errorf("build query: %w", err)
	}
	logger.Debug("sql query", zap.String("query", query), zap.Any("args", args))

	rows, err := p.pgxpool.Query(ctx, query, args...)
	if err != nil {
		logger.Error("couldn't query subtree", zap.Error(err))
		return nil, nil, fmt.Errorf("get subtree: %w", err)
	}
	defer rows.Close()

	var children []domain.Resource
	for rows.Next() {
		r, err := scanResource(rows)
		if err != nil {
			return nil, nil, err
		}
		children = append(children, *r)
	}
	return root, children, rows.Err()
}

func (p *postgres) GetPath(ctx context.Context, uid string) (string, error) {
	logger := p.logger.WithMethod("GetPath")
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
			logger.Warn("entity not found")
			return "", domain.ErrNotFound
		}
		logger.Error("couldn't execute sql query", zap.Error(err))
		return "", err
	}
	return path, nil
}

// ─── helpers ───

func nlevel(path string) int {
	if path == "" {
		return 0
	}
	return strings.Count(path, ".") + 1
}

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
