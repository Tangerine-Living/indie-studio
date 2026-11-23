package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pixelbench/internal/project"
)

func seededStore(t *testing.T) (*project.Store, string) {
	t.Helper()
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())
	store, err := project.Create(project.CreateOptions{
		ParentDir: t.TempDir(),
		Name:      "Godot 导出",
		Engine:    "godot",
		TileSize:  32,
		Template:  "town-square",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	maps, err := store.ListMaps()
	if err != nil || len(maps) == 0 {
		t.Fatalf("seeded maps = %v, %v", maps, err)
	}
	return store, maps[0].ID
}

func TestExportGodotWritesProjectSceneAndTileset(t *testing.T) {
	store, mapID := seededStore(t)
	result, err := ExportGodot(store, mapID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result.Dir, "小镇广场-godot") {
		t.Fatalf("dir = %q", result.Dir)
	}
	if result.Format != "godot" {
		t.Fatalf("format = %q", result.Format)
	}
	want := []string{
		"project.godot",
		"小镇广场.tscn",
		"小镇广场.tres",
		"小镇广场.json",
		"pixelbench_tiles.gd",
		"roof_fade.gd",
		"README.txt",
		"tileset-1-小镇瓦片图集.png",
	}
	for _, name := range want {
		if _, err := os.Stat(filepath.Join(result.Dir, name)); err != nil {
			t.Fatalf("missing %s: %v (files %v)", name, err, result.Files)
		}
	}

	tscn, err := os.ReadFile(filepath.Join(result.Dir, "小镇广场.tscn"))
	if err != nil {
		t.Fatal(err)
	}
	scene := string(tscn)
	if !strings.Contains(scene, "[gd_scene load_steps=") {
		t.Fatal("tscn has no gd_scene header")
	}
	if got := strings.Count(scene, "type=\"TileMapLayer\""); got != 4 {
		t.Fatalf("tile layers = %d, want 4\n%s", got, scene)
	}
	if !strings.Contains(scene, "tile_set = ExtResource(\"tileset\")") {
		t.Fatal("tile layers do not reference the TileSet")
	}
	if !strings.Contains(scene, "layer_id = \"ground\"") {
		t.Fatal("ground layer id missing")
	}
	if got := strings.Count(scene, "type=\"Marker2D\""); got != 4 {
		t.Fatalf("objects = %d, want 4", got)
	}
	if !strings.Contains(scene, "metadata/pb_kind = \"spawn\"") {
		t.Fatal("spawn metadata missing")
	}
	if !strings.Contains(scene, "metadata/pb_properties = {\"dialog\": ") {
		t.Fatal("custom object properties missing")
	}
	if got := strings.Count(scene, "type=\"CollisionShape2D\""); got != 2 {
		t.Fatalf("collision shapes = %d, want 2 merged rectangles", got)
	}
	// First collision block is x 3..9, y 3..8.
	if !strings.Contains(scene, "size = Vector2(224, 192)") {
		t.Fatal("merged collision size missing")
	}
	if !strings.Contains(scene, "position = Vector2(208, 192)") {
		t.Fatal("merged collision center missing")
	}
	if !strings.Contains(scene, "z_index = -20") || !strings.Contains(scene, "z_index = 30") {
		t.Fatal("layer z-order missing")
	}

	tres, err := os.ReadFile(filepath.Join(result.Dir, "小镇广场.tres"))
	if err != nil {
		t.Fatal(err)
	}
	set := string(tres)
	if !strings.Contains(set, "[gd_resource type=\"TileSet\"") {
		t.Fatal("tres is not a TileSet")
	}
	if !strings.Contains(set, "[sub_resource type=\"TileSetAtlasSource\" id=\"atlas_1\"]") {
		t.Fatal("atlas source missing")
	}
	if !strings.Contains(set, "path=\"res://tileset-1-小镇瓦片图集.png\"") {
		t.Fatal("texture resource missing")
	}
	if got := strings.Count(set, "/0 = 0\n"); got != 16 {
		t.Fatalf("declared tiles = %d, want 16", got)
	}
	if got := strings.Count(set, "/0/y_sort_origin = 32"); got != 16 {
		t.Fatalf("y_sort_origin entries = %d, want 16", got)
	}
	if !strings.Contains(set, "sources/0 = SubResource(\"atlas_1\")") {
		t.Fatal("source mapping missing")
	}

	script, err := os.ReadFile(filepath.Join(result.Dir, "pixelbench_tiles.gd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"@tool", "TRANSFORM_FLIP_H", "TRANSFORM_TRANSPOSE", "set_cell("} {
		if !strings.Contains(string(script), marker) {
			t.Fatalf("loader script missing %q", marker)
		}
	}

	proj, err := os.ReadFile(filepath.Join(result.Dir, "project.godot"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proj), "run/main_scene=\"res://小镇广场.tscn\"") {
		t.Fatalf("project.godot main scene missing:\n%s", proj)
	}
}

func TestCollisionRectsMergesSeededBlocks(t *testing.T) {
	store, mapID := seededStore(t)
	m, err := store.LoadMap(mapID)
	if err != nil {
		t.Fatal(err)
	}
	rects := collisionRects(m)
	if len(rects) != 2 {
		t.Fatalf("rects = %+v, want 2", rects)
	}
	if rects[0] != (cellRect{x: 3, y: 3, w: 7, h: 6}) {
		t.Fatalf("first rect = %+v", rects[0])
	}
	if rects[1] != (cellRect{x: 23, y: 8, w: 6, h: 4}) {
		t.Fatalf("second rect = %+v", rects[1])
	}
}

func TestMapTscnAppliesElevationAndVisibility(t *testing.T) {
	m := project.GameMap{
		Name:     "高程",
		TileSize: 32,
		Width:    4,
		Height:   4,
		Settings: project.MapSettings{YSort: true, HeightStep: 16},
		Layers: []project.MapLayer{
			{ID: "ground", Name: "地面", Type: project.MapGround, Visible: true, Opacity: 1},
			{ID: "building", Name: "建筑", Type: project.MapBuilding, Elevation: 2, Visible: false, Opacity: 0.5},
		},
		Objects: []project.MapObject{
			{ID: "o1", Name: "NPC", Kind: "npc", X: 1, Y: 1, W: 1, H: 1, Elevation: 2},
		},
	}
	scene := mapTscn(m, "高程.json", "高程.tres")
	for _, want := range []string{
		"position = Vector2(0, -32)",
		"visible = false",
		"modulate = Color(1, 1, 1, 0.5)",
		"position = Vector2(48, 32)",
		"z_index = 2",
	} {
		if !strings.Contains(scene, want) {
			t.Fatalf("scene missing %q:\n%s", want, scene)
		}
	}
}

func TestGenericJSONKeepsTilesAndFlags(t *testing.T) {
	m := project.GameMap{
		Name: "瓦片", TileSize: 32, Width: 4, Height: 4,
		Layers: []project.MapLayer{{
			ID: "ground", Name: "地面", Type: project.MapGround, Visible: true, Opacity: 1,
			Tiles: []project.MapTile{{X: 2, Y: 3, Set: 0, Tile: 5, FlipH: true, Rotate: 90}},
		}},
	}
	var parsed struct {
		Layers []struct {
			ID    string `json:"id"`
			Tiles []struct {
				X     int  `json:"x"`
				Y     int  `json:"y"`
				Tile  int  `json:"tile"`
				FlipH bool `json:"flipH"`
			} `json:"tiles"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(genericJSON(m), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Layers) != 1 || len(parsed.Layers[0].Tiles) != 1 {
		t.Fatalf("layers = %+v", parsed.Layers)
	}
	tile := parsed.Layers[0].Tiles[0]
	if tile.X != 2 || tile.Y != 3 || tile.Tile != 5 || !tile.FlipH {
		t.Fatalf("tile = %+v", tile)
	}
}

func TestExportMapRejectsUnknownFormat(t *testing.T) {
	store, mapID := seededStore(t)
	if _, err := ExportMap(store, mapID, "unity"); err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}
