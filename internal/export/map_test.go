package export

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pixelbench/internal/project"
)

type tmxMap struct {
	XMLName  xml.Name `xml:"map"`
	Version  string   `xml:"version,attr"`
	Width    int      `xml:"width,attr"`
	Height   int      `xml:"height,attr"`
	TileW    int      `xml:"tilewidth,attr"`
	TileH    int      `xml:"tileheight,attr"`
	Tilesets []struct {
		FirstGID int    `xml:"firstgid,attr"`
		Source   string `xml:"source,attr"`
	} `xml:"tileset"`
	Layers []struct {
		Name string `xml:"name,attr"`
		Data string `xml:"data"`
	} `xml:"layer"`
	Objects []struct {
		Name  string  `xml:"name,attr"`
		X     float64 `xml:"x,attr"`
		Y     float64 `xml:"y,attr"`
		Width float64 `xml:"width,attr"`
		Props []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		} `xml:"properties>property"`
	} `xml:"objectgroup>object"`
}

func TestExportMapWritesTiledAndGenericFiles(t *testing.T) {
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())
	store, err := project.Create(project.CreateOptions{
		ParentDir: t.TempDir(),
		Name:      "导出地图",
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

	result, err := ExportMap(store, maps[0].ID, "tiled")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Dir, filepath.Join("exports", "maps")) {
		t.Fatalf("dir = %q", result.Dir)
	}
	for _, suffix := range []string{".tmx", ".json", ".tsx"} {
		found := false
		for _, file := range result.Files {
			if strings.HasSuffix(file, suffix) {
				found = true
			}
		}
		if !found {
			t.Fatalf("files = %v, missing %s", result.Files, suffix)
		}
	}

	tmxBytes, err := os.ReadFile(filepath.Join(result.Dir, "小镇广场.tmx"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed tmxMap
	if err := xml.Unmarshal(tmxBytes, &parsed); err != nil {
		t.Fatalf("tmx is not valid XML: %v", err)
	}
	if parsed.Width != 40 || parsed.Height != 30 || parsed.TileW != 32 {
		t.Fatalf("map = %dx%d @ %d", parsed.Width, parsed.Height, parsed.TileW)
	}
	if len(parsed.Tilesets) != 1 || parsed.Tilesets[0].FirstGID != 1 {
		t.Fatalf("tilesets = %+v", parsed.Tilesets)
	}
	if len(parsed.Layers) != 5 {
		t.Fatalf("layers = %d, want 5 tile/metadata layers", len(parsed.Layers))
	}
	rows := strings.Split(strings.TrimSpace(parsed.Layers[0].Data), "\n")
	if len(rows) != 30 {
		t.Fatalf("ground rows = %d", len(rows))
	}
	firstRow := strings.Split(rows[0], ",")
	if len(firstRow) != 40 {
		t.Fatalf("ground columns = %d", len(firstRow))
	}
	if len(parsed.Objects) != 4 {
		t.Fatalf("objects = %d", len(parsed.Objects))
	}
	spawn := parsed.Objects[0]
	if spawn.Name != "玩家出生点" || spawn.X != 19.5*32 || spawn.Width != 32 {
		t.Fatalf("spawn = %+v", spawn)
	}
	if len(spawn.Props) == 0 {
		t.Fatal("object properties missing")
	}

	tsxBytes, err := os.ReadFile(filepath.Join(result.Dir, "tileset-1-小镇瓦片图集.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	var tsx struct {
		TileCount int `xml:"tilecount,attr"`
		Columns   int `xml:"columns,attr"`
		Image     struct {
			Source string `xml:"source,attr"`
			Width  int    `xml:"width,attr"`
		} `xml:"image"`
	}
	if err := xml.Unmarshal(tsxBytes, &tsx); err != nil {
		t.Fatal(err)
	}
	if tsx.TileCount != 16 || tsx.Columns != 4 || tsx.Image.Width != 128 {
		t.Fatalf("tsx = %+v", tsx)
	}
	if _, err := os.Stat(filepath.Join(result.Dir, tsx.Image.Source)); err != nil {
		t.Fatalf("tileset image missing: %v", err)
	}

	genericBytes, err := os.ReadFile(filepath.Join(result.Dir, "小镇广场.json"))
	if err != nil {
		t.Fatal(err)
	}
	var generic struct {
		Format    string  `json:"format"`
		Collision [][]int `json:"collision"`
		Layers    []struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Tiles []struct {
				X     int  `json:"x"`
				Y     int  `json:"y"`
				Set   int  `json:"set"`
				Tile  int  `json:"tile"`
				FlipH bool `json:"flipH"`
			} `json:"tiles"`
		} `json:"layers"`
		Objects []struct {
			Name  string `json:"name"`
			Depth int    `json:"depth"`
		} `json:"objects"`
		Settings struct {
			YSort      bool `json:"ySort"`
			RoofFade   bool `json:"roofFade"`
			HeightStep int  `json:"heightStep"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(genericBytes, &generic); err != nil {
		t.Fatal(err)
	}
	if generic.Format != "pixelbench-map" || len(generic.Collision) != 30 || len(generic.Collision[0]) != 40 {
		t.Fatalf("generic = %+v", generic)
	}
	if generic.Collision[4][5] != 1 {
		t.Fatal("pond collision missing")
	}
	if !generic.Settings.YSort || !generic.Settings.RoofFade || generic.Settings.HeightStep != 16 {
		t.Fatalf("settings = %+v", generic.Settings)
	}
	if generic.Objects[0].Depth != 560 {
		t.Fatalf("spawn depth = %d", generic.Objects[0].Depth)
	}
	if len(generic.Layers) != 6 {
		t.Fatalf("generic layers = %d, want 6", len(generic.Layers))
	}
	var groundTiles int
	for _, layer := range generic.Layers {
		if layer.ID == "ground" {
			groundTiles = len(layer.Tiles)
		}
	}
	if groundTiles == 0 {
		t.Fatal("generic JSON dropped the ground tiles the Godot loader needs")
	}
}

func TestExportMapRejectsMissingTilesetAsset(t *testing.T) {
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())
	store, err := project.Create(project.CreateOptions{
		ParentDir: t.TempDir(),
		Name:      "缺素材",
		Engine:    "godot",
		TileSize:  32,
		Template:  "town-square",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := project.GameMap{
		ID: "missing-set", Name: "缺素材地图", TileSize: 32, Width: 4, Height: 4,
		Tilesets: []project.MapTileset{{ID: "set", AssetID: "does-not-exist", TileSize: 32, Columns: 1, TileCount: 1}},
		Layers:   []project.MapLayer{{ID: "ground", Name: "地面", Type: project.MapGround, Visible: true, Opacity: 1}},
		Settings: project.MapSettings{YSort: true, RoofFade: true, HeightStep: 16, Ambient: "#8090a0"},
	}
	saved, err := store.SaveMap(m)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ExportMap(store, saved.ID, "tiled")
	if err == nil {
		t.Fatal("expected missing tileset asset error")
	}
}
