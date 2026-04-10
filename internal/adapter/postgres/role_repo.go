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

type RoleRepo struct {
	db *pgxpool.Pool
}

func NewRoleRepo(db *pgxpool.Pool) *RoleRepo {
	return &RoleRepo{db: db}
}

func (repo *RoleRepo) Add(ctx context.Context, r *domain.Role) error {
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

	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO roles
			(uid, access_object_uid, parent_role_uid, name, display_name, description, permissions, attributes, labels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		r.UID, r.AccessObjectUID, parentUID, r.Name, r.DisplayName, r.Description, perms, attrs, labels,
	)
	if err != nil {
		return fmt.Errorf("insert role: %w", err)
	}

	if err := insertRoleResources(ctx, tx, r.UID, r.ResourceUIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (repo *RoleRepo) GetByUID(ctx context.Context, uid string) (*domain.Role, error) {
	row := repo.db.QueryRow(ctx, `
		SELECT uid, access_object_uid, COALESCE(parent_role_uid,''), name,
		       display_name, description, permissions, attributes, labels, source
		FROM roles
		WHERE uid = $1`, uid)

	r, err := scanRole(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("role %s: %w", uid, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("scan role: %w", err)
	}

	r.ResourceUIDs, err = repo.getResourceUIDs(ctx, uid)
	if err != nil {
		return nil, err
	}

	r.Children, err = repo.getChildren(ctx, uid)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (repo *RoleRepo) List(ctx context.Context, f domain.RoleFilter) ([]domain.Role, int32, error) {
	var conds []string
	var args []any

	if f.AccessObjectUID != "" {
		args = append(args, f.AccessObjectUID)
		conds = append(conds, fmt.Sprintf(`r.access_object_uid = $%d`, len(args)))
	}

	var joinClause string
	if f.ResourceUID != "" {
		args = append(args, f.ResourceUID)
		joinClause = fmt.Sprintf(` INNER JOIN role_resources rr ON rr.role_uid = r.uid AND rr.resource_uid = $%d`, len(args))
	}

	// List only top-level roles
	conds = append(conds, `r.parent_role_uid IS NULL`)

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int32
	if err := repo.db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT r.uid) FROM roles r`+joinClause+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count roles: %w", err)
	}

	args = append(args, f.PageSize, pageOffset(f.Page, f.PageSize))
	rows, err := repo.db.Query(ctx, `
		SELECT DISTINCT r.uid, r.access_object_uid, COALESCE(r.parent_role_uid,''), r.name,
		       r.display_name, r.description, r.permissions, r.attributes, r.labels, r.source
		FROM roles r`+joinClause+where+
		fmt.Sprintf(` ORDER BY r.name LIMIT $%d OFFSET $%d`, len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var result []domain.Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, 0, err
		}
		r.ResourceUIDs, err = repo.getResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, 0, err
		}
		r.Children, err = repo.getChildren(ctx, r.UID)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *r)
	}
	return result, total, rows.Err()
}

func (repo *RoleRepo) Update(ctx context.Context, uid string, upd domain.RoleUpdate) (*domain.Role, error) {
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

	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		UPDATE roles
		SET display_name = $2, description = $3, permissions = $4, attributes = $5, labels = $6, updated_at = NOW()
		WHERE uid = $1`,
		uid, upd.DisplayName, upd.Description, perms, attrs, labels,
	)
	if err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}

	// Replace resource associations
	if _, err := tx.Exec(ctx, `DELETE FROM role_resources WHERE role_uid = $1`, uid); err != nil {
		return nil, fmt.Errorf("clear role resources: %w", err)
	}
	if err := insertRoleResources(ctx, tx, uid, upd.ResourceUIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return repo.GetByUID(ctx, uid)
}

func (repo *RoleRepo) Remove(ctx context.Context, uid string) error {
	tag, err := repo.db.Exec(ctx, `DELETE FROM roles WHERE uid = $1`, uid)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("role %s: %w", uid, domain.ErrNotFound)
	}
	return nil
}

func (repo *RoleRepo) getResourceUIDs(ctx context.Context, roleUID string) ([]string, error) {
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

func (repo *RoleRepo) getChildren(ctx context.Context, parentUID string) ([]domain.Role, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT uid, access_object_uid, COALESCE(parent_role_uid,''), name,
		       display_name, description, permissions, attributes, labels, source
		FROM roles
		WHERE parent_role_uid = $1`, parentUID)
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
		r.ResourceUIDs, err = repo.getResourceUIDs(ctx, r.UID)
		if err != nil {
			return nil, err
		}
		result = append(result, *r)
	}
	return result, rows.Err()
}

// ─── helpers ───

func insertRoleResources(ctx context.Context, tx pgx.Tx, roleUID string, resourceUIDs []string) error {
	for _, rUID := range resourceUIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO role_resources (role_uid, resource_uid) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			roleUID, rUID,
		); err != nil {
			return fmt.Errorf("insert role_resource: %w", err)
		}
	}
	return nil
}

func scanRole(s interface {
	Scan(...any) error
}) (*domain.Role, error) {
	var permsB, attrsB, labelsB, srcB []byte
	r := &domain.Role{}

	err := s.Scan(
		&r.UID, &r.AccessObjectUID, &r.ParentRoleUID, &r.Name,
		&r.DisplayName, &r.Description, &permsB, &attrsB, &labelsB, &srcB,
	)
	if err != nil {
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
