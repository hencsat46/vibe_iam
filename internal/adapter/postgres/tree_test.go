package postgres

import (
	"testing"

	"temp/internal/domain"
)

func TestBuildResourceTree(t *testing.T) {
	flat := []domain.Resource{
		{UID: "server-1", ParentUID: "",         Name: "pg-main",  Path: "pg_main"},
		{UID: "db-1",     ParentUID: "server-1", Name: "rez_dev",  Path: "pg_main.rez_dev"},
		{UID: "table-1",  ParentUID: "db-1",     Name: "users",    Path: "pg_main.rez_dev.users"},
		{UID: "table-2",  ParentUID: "db-1",     Name: "orders",   Path: "pg_main.rez_dev.orders"},
	}

	tree := buildResourceTree(flat)

	if len(tree) != 1 {
		t.Fatalf("expected 1 root, got %d", len(tree))
	}
	root := tree[0]
	if root.Name != "pg-main" {
		t.Errorf("expected root pg-main, got %s", root.Name)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child of pg-main, got %d", len(root.Children))
	}
	db := root.Children[0]
	if db.Name != "rez_dev" {
		t.Errorf("expected rez_dev, got %s", db.Name)
	}
	if len(db.Children) != 2 {
		t.Fatalf("expected 2 children of rez_dev, got %d", len(db.Children))
	}
}
