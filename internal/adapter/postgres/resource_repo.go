package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"temp/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ResourceRepo struct {
	db *pgxpool.Pool
}

func NewResourceRepo(db *pgxpool.Pool) *ResourceRepo {
	return &ResourceRepo{db: db}
}

func (repo *ResourceRepo) Add(ctx context.Context, r *domain.Resource) error {
	attrs, err := json.Marshal(r.Attributes)
	if err != nil {
		return fmt.Errorf("marshal attributes: %w", err)
	}

	var parentUID *string
	if r.ParentUID != "" {
		parentUID = &r.ParentUID
	}

	_, err = repo.db.Exec(ctx, `
		INSERT INTO resources
			(uid, access_object_uid, parent_uid, resource_type, name, display_name, description, path, attributes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::ltree, $9)`,
		r.UID, r.AccessObjectUID, parentUID, r.ResourceType, r.Name, r.DisplayName, r.Description, r.Path, attrs,
	)
	if err != nil {
		return fmt.Errorf("insert resource: %w", err)
	}
	return nil
}

func (repo *ResourceRepo) GetByUID(ctx context.Context, uid string) (*domain.Resource, error) {
	row := repo.db.QueryRow(ctx, `
		SELECT uid, access_object_uid, COALESCE(parent_uid,''), resource_type, name,
		       display_name, description, path::text, attributes, source
		FROM resources
		WHERE uid = $1`, uid)

	r, err := scanResource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("resource %s: %w", uid, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("scan resource: %w", err)
	}
	return r, nil
}

func (repo *ResourceRepo) List(ctx context.Context, f domain.ResourceFilter) ([]domain.Resource, int32, error) {
	var conds []string
	var args []any

	if f.AccessObjectUID != "" {
		args = append(args, f.AccessObjectUID)
		conds = append(conds, fmt.Sprintf(`access_object_uid = $%d`, len(args)))
	}
	if f.ParentUID != "" {
		args = append(args, f.ParentUID)
		conds = append(conds, fmt.Sprintf(`parent_uid = $%d`, len(args)))
	} else if f.AccessObjectUID != "" {
		// Default to root resources when no parent specified
		conds = append(conds, `parent_uid IS NULL`)
	}
	if f.ResourceType != "" {
		args = append(args, f.ResourceType)
		conds = append(conds, fmt.Sprintf(`resource_type = $%d`, len(args)))
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int32
	if err := repo.db.QueryRow(ctx, `SELECT COUNT(*) FROM resources`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count resources: %w", err)
	}

	args = append(args, f.PageSize, pageOffset(f.Page, f.PageSize))
	rows, err := repo.db.Query(ctx, `
		SELECT uid, access_object_uid, COALESCE(parent_uid,''), resource_type, name,
		       display_name, description, path::text, attributes, source
		FROM resources`+where+
		fmt.Sprintf(` ORDER BY path LIMIT $%d OFFSET $%d`, len(args)-1, len(args)),
		args...,
	)
	if err != nil {
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

func (repo *ResourceRepo) Update(ctx context.Context, uid string, upd domain.ResourceUpdate) (*domain.Resource, error) {
	attrs, err := json.Marshal(upd.Attributes)
	if err != nil {
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}

	_, err = repo.db.Exec(ctx, `
		UPDATE resources
		SET display_name = $2, description = $3, attributes = $4, updated_at = NOW()
		WHERE uid = $1`,
		uid, upd.DisplayName, upd.Description, attrs,
	)
	if err != nil {
		return nil, fmt.Errorf("update resource: %w", err)
	}
	return repo.GetByUID(ctx, uid)
}

func (repo *ResourceRepo) Remove(ctx context.Context, uid string) error {
	// Cascade handled by DB (ON DELETE CASCADE on parent_uid FK with DEFERRABLE)
	// But ltree path subtree deletion needs manual handling or triggers.
	// We delete the subtree by path prefix.
	row := repo.db.QueryRow(ctx, `SELECT path::text FROM resources WHERE uid = $1`, uid)
	var path string
	if err := row.Scan(&path); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("resource %s: %w", uid, domain.ErrNotFound)
		}
		return err
	}

	_, err := repo.db.Exec(ctx, `DELETE FROM resources WHERE path <@ $1::ltree`, path)
	if err != nil {
		return fmt.Errorf("remove resource subtree: %w", err)
	}
	return nil
}

func (repo *ResourceRepo) GetSubtree(ctx context.Context, uid string, maxDepth int32) (*domain.Resource, []domain.Resource, error) {
	root, err := repo.GetByUID(ctx, uid)
	if err != nil {
		return nil, nil, err
	}

	args := []any{root.Path, uid}
	q := `
		SELECT uid, access_object_uid, COALESCE(parent_uid,''), resource_type, name,
		       display_name, description, path::text, attributes, source
		FROM resources
		WHERE path <@ $1::ltree AND uid != $2`

	if maxDepth > 0 {
		args = append(args, nlevel(root.Path)+int(maxDepth))
		q += fmt.Sprintf(` AND nlevel(path) <= $%d`, len(args))
	}
	q += ` ORDER BY path`

	rows, err := repo.db.Query(ctx, q, args...)
	if err != nil {
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

func (repo *ResourceRepo) GetPath(ctx context.Context, uid string) (string, error) {
	var path string
	err := repo.db.QueryRow(ctx, `SELECT path::text FROM resources WHERE uid = $1`, uid).Scan(&path)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("resource %s: %w", uid, domain.ErrNotFound)
		}
		return "", err
	}
	return path, nil
}

// nlevel counts the number of labels in an ltree path.
func nlevel(path string) int {
	if path == "" {
		return 0
	}
	return strings.Count(path, ".") + 1
}

// scanResource scans a resource row from either pgx.Row or pgx.Rows.
func scanResource(s interface {
	Scan(...any) error
}) (*domain.Resource, error) {
	var attrsB, srcB []byte
	r := &domain.Resource{}

	err := s.Scan(
		&r.UID, &r.AccessObjectUID, &r.ParentUID, &r.ResourceType, &r.Name,
		&r.DisplayName, &r.Description, &r.Path, &attrsB, &srcB,
	)
	if err != nil {
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
