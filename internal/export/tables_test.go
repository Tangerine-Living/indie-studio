package export

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/project"
)

func TestExportTableWritesJSONAndCSV(t *testing.T) {
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())
	store, err := project.Create(project.CreateOptions{
		ParentDir: t.TempDir(),
		Name:      "导出数值表",
		Engine:    "godot",
		TileSize:  32,
		Template:  "town-square",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	tables, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	var enemies, items project.DataTable
	for _, table := range tables {
		switch table.Key {
		case "enemies":
			enemies = table
		case "items":
			items = table
		}
	}
	if enemies.ID == "" || items.ID == "" {
		t.Fatalf("seeded tables missing: %v", tables)
	}

	result, err := ExportTable(store, enemies.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(result.Dir), "/exports/tables/enemies") {
		t.Fatalf("dir = %q", result.Dir)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files = %v", result.Files)
	}

	raw, err := os.ReadFile(filepath.Join(result.Dir, "enemies.json"))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Key     string `json:"key"`
		Columns []struct {
			Key      string `json:"key"`
			RefTable string `json:"refTable"`
		} `json:"columns"`
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Key != "enemies" || len(payload.Rows) != 3 {
		t.Fatalf("payload = %+v", payload)
	}
	refSeen := false
	for _, col := range payload.Columns {
		if col.Key == "drop" && col.RefTable == "items" {
			refSeen = true
		}
	}
	if !refSeen {
		t.Fatalf("drop column lost its refTable: %+v", payload.Columns)
	}
	first := payload.Rows[0]
	if first["id"] == "" || first["name"] == "" {
		t.Fatalf("first row = %v", first)
	}
	if _, ok := first["hp"].(float64); !ok {
		t.Fatalf("hp should export as a number, got %#v", first["hp"])
	}

	file, err := os.Open(filepath.Join(result.Dir, "enemies.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4 {
		t.Fatalf("csv rows = %d, want header + 3", len(records))
	}
	if got := strings.Join(records[0], ","); got != "id,name,hp,attack,speed,drop" {
		t.Fatalf("csv header = %q", got)
	}
}

func TestExportTableBlocksOnDanglingReference(t *testing.T) {
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())
	store, err := project.Create(project.CreateOptions{
		ParentDir: t.TempDir(),
		Name:      "数值表引用",
		Engine:    "godot",
		TileSize:  32,
		Template:  "town-square",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	tables, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	var enemies, items project.DataTable
	for _, table := range tables {
		switch table.Key {
		case "enemies":
			enemies = table
		case "items":
			items = table
		}
	}

	// 面包 is the drop of 野狗; removing it must block the enemy export.
	kept := items.Rows[:0]
	for _, row := range items.Rows {
		isBread := false
		for _, col := range items.Columns {
			if col.Key == "name" && row.Values[col.ID] == "面包" {
				isBread = true
			}
		}
		if !isBread {
			kept = append(kept, row)
		}
	}
	items.Rows = kept
	if _, err := store.SaveTable(items); err != nil {
		t.Fatal(err)
	}

	if _, err := ExportTable(store, enemies.ID); !apperr.Is(err, "TABLE_INVALID") {
		t.Fatalf("export = %v, want TABLE_INVALID", err)
	}
}