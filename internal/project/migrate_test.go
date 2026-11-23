package project

import (
	"os"
	"path/filepath"
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
)

func TestMigrateFromZero(t *testing.T) {
	testConfig(t)
	root := filepath.Join(t.TempDir(), "旧项目")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{
		"schemaVersion": 0,
		"name":          "旧项目",
		"settings":      map[string]any{"tileSize": 16},
	}
	if err := fsutil.WriteJSONAtomic(ProjectFile(root), legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := Open(root, false)
	if err != nil {
		t.Fatalf("Open legacy: %v", err)
	}
	defer store.Close()

	session := store.Session()
	if session.Project.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %d, want %d", session.Project.SchemaVersion, SchemaVersion)
	}
	if session.Project.Settings.TileSize != 16 {
		t.Fatalf("tile size = %d, want 16", session.Project.Settings.TileSize)
	}
	if session.Project.ID == "" || session.Project.Engine != "godot" {
		t.Fatalf("migrated project = %+v", session.Project)
	}

	backups, err := filepath.Glob(filepath.Join(WorkbenchDir(root), "backups", "project-v0-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups = %v, want one", backups)
	}
}

func TestRejectsFutureSchema(t *testing.T) {
	testConfig(t)
	root := filepath.Join(t.TempDir(), "未来项目")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	future := map[string]any{"schemaVersion": 999, "name": "未来项目"}
	if err := fsutil.WriteJSONAtomic(ProjectFile(root), future, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, false); !apperr.Is(err, "SCHEMA_TOO_NEW") {
		t.Fatalf("error = %v, want SCHEMA_TOO_NEW", err)
	}
}
