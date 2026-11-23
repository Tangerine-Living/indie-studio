package project

import (
	"os"
	"path/filepath"
	"testing"
)

func validMap() GameMap {
	return GameMap{
		ID:       "test-map",
		Name:     "测试地图",
		TileSize: 32,
		Width:    8,
		Height:   6,
		Tilesets: []MapTileset{{
			ID: "set", AssetID: "asset-1", TileSize: 32, Columns: 4, TileCount: 16,
		}},
		Layers: []MapLayer{
			{ID: "ground", Name: "地面", Type: MapGround, Visible: true, Opacity: 1, Tiles: []MapTile{
				{X: 0, Y: 0, Set: 0, Tile: 1},
				{X: 0, Y: 0, Set: 0, Tile: 2}, // duplicate: last wins
				{X: -1, Y: 0, Set: 0, Tile: 1},
				{X: 99, Y: 0, Set: 0, Tile: 1},
				{X: 1, Y: 0, Set: 3, Tile: 1},
				{X: 2, Y: 0, Set: 0, Tile: 99},
			}},
			{ID: "objects", Name: "对象", Type: MapLayerObject, Visible: true, Opacity: 1},
		},
		Objects: []MapObject{{
			ID: "spawn", Name: "出生点", Kind: MapObjectSpawn, X: 2, Y: 3, W: 1, H: 1,
			YSort: true, Color: "#ffffff",
		}},
		Settings: MapSettings{YSort: true, RoofFade: true, HeightStep: 16, Ambient: "#8090a0"},
	}
}

func TestValidateMapNormalizesSparseTiles(t *testing.T) {
	m := validMap()
	if err := ValidateMap(&m); err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %d", m.SchemaVersion)
	}
	if len(m.Layers[0].Tiles) != 1 || m.Layers[0].Tiles[0].Tile != 2 {
		t.Fatalf("tiles = %+v, want one deduplicated cell", m.Layers[0].Tiles)
	}
	if m.Settings.RoofFade != true || m.Settings.HeightStep != 16 {
		t.Fatalf("settings = %+v", m.Settings)
	}
}

func TestExpandMapShiftsContent(t *testing.T) {
	m := validMap()
	if err := ValidateMap(&m); err != nil {
		t.Fatal(err)
	}
	if err := ExpandMap(&m, 2, 3, 0, 1); err != nil {
		t.Fatalf("ExpandMap: %v", err)
	}
	if m.Width != 12 || m.Height != 8 {
		t.Fatalf("size = %d×%d, want 12×8", m.Width, m.Height)
	}
	if len(m.Layers[0].Tiles) != 1 {
		t.Fatalf("tiles = %+v", m.Layers[0].Tiles)
	}
	if tile := m.Layers[0].Tiles[0]; tile.X != 1 || tile.Y != 2 {
		t.Fatalf("tile = %+v, want shifted to 1,2", tile)
	}
	if m.Objects[0].X != 3 || m.Objects[0].Y != 5 {
		t.Fatalf("object = %+v, want shifted to 3,5", m.Objects[0])
	}
	if err := ExpandMap(&m, 0, 0, 0, 0); err == nil {
		t.Fatal("expected zero expansion to be rejected")
	}
	if err := ExpandMap(&m, 0, 5000, 0, 0); err == nil {
		t.Fatal("expected oversized expansion to be rejected")
	}
}

func TestValidateMapRejectsBadStructure(t *testing.T) {
	badLayer := validMap()
	badLayer.Layers[0].Type = "magic"
	if err := ValidateMap(&badLayer); err == nil {
		t.Fatal("expected invalid layer type")
	}

	badObject := validMap()
	badObject.Objects[0].Kind = "dragon"
	if err := ValidateMap(&badObject); err == nil {
		t.Fatal("expected invalid object kind")
	}

	tooBig := validMap()
	tooBig.Width = 2048
	if err := ValidateMap(&tooBig); err == nil {
		t.Fatal("expected map size error")
	}
}

func TestMapSaveLoadDeleteRoundTrip(t *testing.T) {
	store := newTestStore(t, "地图项目")
	m := validMap()
	if err := ValidateMap(&m); err != nil {
		t.Fatal(err)
	}
	saved, err := store.SaveMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if saved.UpdatedAt.IsZero() {
		t.Fatal("save did not stamp updatedAt")
	}
	if _, err := os.Stat(MapPath(store.Root(), m.ID)); err != nil {
		t.Fatalf("map file missing: %v", err)
	}
	loaded, err := store.LoadMap(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != m.Name || len(loaded.Layers[0].Tiles) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}
	if err := store.DeleteMap(m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadMap(m.ID); err == nil {
		t.Fatal("deleted map still loads")
	}
	if _, err := os.Stat(filepath.Join(WorkbenchDir(store.Root()), "trash", "maps")); err != nil {
		t.Fatalf("trash folder missing: %v", err)
	}
}
