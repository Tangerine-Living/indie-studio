package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// ExportGodot writes a self-contained Godot 4 folder under
// exports/maps/<name>-godot/. The folder doubles as a minimal Godot project so
// the scene can be opened and run directly; its res:// paths are relative to
// the exported folder itself.
//
// The TileSet lives in a .tres file. TileMapLayer cell data is not written as
// Godot's packed binary blob (whose layout is engine-version specific);
// instead each layer references the generic JSON sidecar through a small
// @tool script that calls set_cell. Objects and collision shapes are real
// scene nodes.
func ExportGodot(store *project.Store, mapID string) (MapExportResult, error) {
	m, err := store.LoadMap(mapID)
	if err != nil {
		return MapExportResult{}, err
	}
	assets, err := store.ListAssets()
	if err != nil {
		return MapExportResult{}, err
	}
	byID := make(map[string]project.Asset, len(assets))
	for _, asset := range assets {
		byID[asset.ID] = asset
	}

	base := safeName(m.Name)
	dir := filepath.Join(project.ExportsDir(store.Root()), "maps", base+"-godot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return MapExportResult{}, apperr.Wrap("IO", "无法创建 Godot 导出目录", err)
	}
	result := MapExportResult{Dir: dir, Files: []string{}, Format: "godot"}
	write := func(name string, data []byte) error {
		if err := fsutil.WriteFileAtomic(filepath.Join(dir, name), data, 0o644); err != nil {
			return apperr.Wrap("IO", "无法写入导出文件", err).WithDetail(name)
		}
		result.Files = append(result.Files, name)
		return nil
	}

	type godotTileset = tilesetGodotInfo
	infos := make([]tilesetGodotInfo, len(m.Tilesets))
	for i, set := range m.Tilesets {
		asset, ok := byID[set.AssetID]
		if !ok {
			return MapExportResult{}, apperr.New("MAP_TILESET_MISSING", "地图引用的瓦片集素材不存在").WithDetail(set.AssetID)
		}
		imageFile, err := assetImagePath(store, asset.ID)
		if err != nil {
			return MapExportResult{}, err
		}
		imageName := fmt.Sprintf("tileset-%d-%s%s", i+1, safeName(asset.Name), filepath.Ext(imageFile))
		if err := copyInto(dir, imageFile, imageName); err != nil {
			return MapExportResult{}, err
		}
		result.Files = append(result.Files, imageName)
		infos[i] = godotTileset{image: imageName, asset: asset}
	}

	jsonName := base + ".json"
	if err := write(jsonName, genericJSON(m)); err != nil {
		return MapExportResult{}, err
	}
	tresName := base + ".tres"
	if err := write(tresName, []byte(tilesetTres(m, infos))); err != nil {
		return MapExportResult{}, err
	}
	sceneName := base + ".tscn"
	if err := write(sceneName, []byte(mapTscn(m, jsonName, tresName))); err != nil {
		return MapExportResult{}, err
	}
	if err := write("pixelbench_tiles.gd", []byte(tileLoaderScript)); err != nil {
		return MapExportResult{}, err
	}
	if err := write("roof_fade.gd", []byte(roofFadeScript)); err != nil {
		return MapExportResult{}, err
	}
	if err := write("README.txt", []byte(godotReadme(m, sceneName))); err != nil {
		return MapExportResult{}, err
	}
	if err := write("project.godot", []byte(projectGodot(m, sceneName))); err != nil {
		return MapExportResult{}, err
	}
	return result, nil
}

func tilesetTres(m project.GameMap, infos []tilesetGodotInfo) string {
	var b strings.Builder
	loadSteps := len(infos) * 2 // texture + atlas source
	b.WriteString(fmt.Sprintf("[gd_resource type=\"TileSet\" load_steps=%d format=3]\n\n", loadSteps+1))
	for i, info := range infos {
		fmt.Fprintf(&b, "[ext_resource type=\"Texture2D\" path=\"res://%s\" id=\"tex_%d\"]\n", tscnString(info.image), i+1)
	}
	b.WriteString("\n")
	for i, set := range m.Tilesets {
		fmt.Fprintf(&b, "[sub_resource type=\"TileSetAtlasSource\" id=\"atlas_%d\"]\n", i+1)
		fmt.Fprintf(&b, "texture = ExtResource(\"tex_%d\")\n", i+1)
		fmt.Fprintf(&b, "texture_region_size = Vector2i(%d, %d)\n", set.TileSize, set.TileSize)
		columns := set.Columns
		if columns < 1 {
			columns = 1
		}
		for index := 0; index < set.TileCount; index++ {
			x := index % columns
			y := index / columns
			fmt.Fprintf(&b, "%d:%d/0 = 0\n", x, y)
			// Top-down tiles sort by their bottom edge, matching the editor.
			fmt.Fprintf(&b, "%d:%d/0/y_sort_origin = %d\n", x, y, set.TileSize)
		}
		b.WriteString("\n")
	}
	b.WriteString("[resource]\n")
	fmt.Fprintf(&b, "tile_size = Vector2i(%d, %d)\n", m.TileSize, m.TileSize)
	for i := range m.Tilesets {
		fmt.Fprintf(&b, "sources/%d = SubResource(\"atlas_%d\")\n", i, i+1)
	}
	return b.String()
}

// tilesetGodotInfo is the exported image for one map tileset.
type tilesetGodotInfo struct {
	image string
	asset project.Asset
}

// layerZ keeps the editor's fixed draw order: ground behind, roof in front.
func layerZ(kind string) int {
	switch kind {
	case project.MapGround:
		return -20
	case project.MapDecoration:
		return -10
	case project.MapBuilding:
		return 0
	case project.MapRoof:
		return 30
	case project.MapLight:
		return 40
	default:
		return 0
	}
}

func mapTscn(m project.GameMap, jsonName, tresName string) string {
	base := safeName(m.Name)
	var b strings.Builder
	tileLayers := make([]project.MapLayer, 0, len(m.Layers))
	for _, layer := range m.Layers {
		if layer.Type == project.MapCollision || layer.Type == project.MapLayerObject {
			continue
		}
		tileLayers = append(tileLayers, layer)
	}
	rects := collisionRects(m)
	subStart := 1
	loadSteps := 2 + len(rects) // tileset + tiles script + rectangle shapes
	b.WriteString(fmt.Sprintf("[gd_scene load_steps=%d format=3]\n\n", loadSteps+1))
	fmt.Fprintf(&b, "[ext_resource type=\"TileSet\" path=\"res://%s\" id=\"tileset\"]\n", tscnString(tresName))
	b.WriteString("[ext_resource type=\"Script\" path=\"res://pixelbench_tiles.gd\" id=\"tiles\"]\n\n")
	for i, rect := range rects {
		fmt.Fprintf(&b, "[sub_resource type=\"RectangleShape2D\" id=\"rect_%d\"]\n", i+1)
		fmt.Fprintf(&b, "size = Vector2(%d, %d)\n\n", rect.w*m.TileSize, rect.h*m.TileSize)
	}
	fmt.Fprintf(&b, "[node name=\"%s\" type=\"Node2D\"]\n", tscnString(base))
	fmt.Fprintf(&b, "y_sort_enabled = %s\n\n", godotBool(m.Settings.YSort))

	for _, layer := range tileLayers {
		fmt.Fprintf(&b, "[node name=\"%s\" type=\"TileMapLayer\" parent=\".\"]\n", tscnString(safeNodeName(layer.Name)))
		if layer.Elevation != 0 {
			fmt.Fprintf(&b, "position = Vector2(0, %d)\n", -layer.Elevation*m.Settings.HeightStep)
		}
		fmt.Fprintf(&b, "z_index = %d\n", layerZ(layer.Type))
		fmt.Fprintf(&b, "visible = %s\n", godotBool(layer.Visible))
		if layer.Opacity > 0 && layer.Opacity < 1 {
			fmt.Fprintf(&b, "modulate = Color(1, 1, 1, %g)\n", layer.Opacity)
		}
		if m.Settings.YSort {
			b.WriteString("y_sort_enabled = true\n")
		}
		b.WriteString("tile_set = ExtResource(\"tileset\")\n")
		b.WriteString("script = ExtResource(\"tiles\")\n")
		fmt.Fprintf(&b, "cells_path = \"res://%s\"\n", tscnString(jsonName))
		fmt.Fprintf(&b, "layer_id = \"%s\"\n\n", tscnString(layer.ID))
	}

	if len(m.Objects) > 0 {
		b.WriteString("[node name=\"Objects\" type=\"Node2D\" parent=\".\"]\n")
		fmt.Fprintf(&b, "y_sort_enabled = %s\n", godotBool(m.Settings.YSort))
		b.WriteString("z_index = 10\n\n")
		used := map[string]int{}
		for _, obj := range m.Objects {
			name := uniqueNodeName(safeNodeName(obj.Name), used)
			fmt.Fprintf(&b, "[node name=\"%s\" type=\"Marker2D\" parent=\"Objects\"]\n", tscnString(name))
			// Bottom-centre is the sort anchor, matching objectDepth in the editor.
			px := (obj.X + obj.W/2) * float64(m.TileSize)
			py := (obj.Y+obj.H)*float64(m.TileSize) - float64(obj.Elevation*m.Settings.HeightStep)
			fmt.Fprintf(&b, "position = Vector2(%g, %g)\n", px, py)
			if obj.Elevation != 0 {
				fmt.Fprintf(&b, "z_index = %d\n", obj.Elevation)
			}
			if m.Settings.YSort {
				b.WriteString("y_sort_enabled = true\n")
			}
			fmt.Fprintf(&b, "metadata/pb_kind = \"%s\"\n", tscnString(obj.Kind))
			fmt.Fprintf(&b, "metadata/pb_id = \"%s\"\n", tscnString(obj.ID))
			fmt.Fprintf(&b, "metadata/pb_elevation = %d\n", obj.Elevation)
			fmt.Fprintf(&b, "metadata/pb_solid = %s\n", godotBool(obj.Solid))
			fmt.Fprintf(&b, "metadata/pb_roof_fade = %s\n", godotBool(obj.RoofFade))
			fmt.Fprintf(&b, "metadata/pb_depth = %d\n", objectDepth(m, obj))
			if obj.AssetID != "" {
				fmt.Fprintf(&b, "metadata/pb_asset_id = \"%s\"\n", tscnString(obj.AssetID))
			}
			if len(obj.Properties) > 0 {
				fmt.Fprintf(&b, "metadata/pb_properties = %s\n", godotDict(obj.Properties))
			}
			b.WriteString("\n")
		}
	}

	if len(rects) > 0 {
		b.WriteString("[node name=\"Collision\" type=\"StaticBody2D\" parent=\".\"]\n")
		b.WriteString("collision_layer = 1\n")
		b.WriteString("collision_mask = 0\n\n")
		for i, rect := range rects {
			name := fmt.Sprintf("Shape%03d", i+1)
			fmt.Fprintf(&b, "[node name=\"%s\" type=\"CollisionShape2D\" parent=\"Collision\"]\n", name)
			cx := (float64(rect.x) + float64(rect.w)/2) * float64(m.TileSize)
			cy := (float64(rect.y) + float64(rect.h)/2) * float64(m.TileSize)
			fmt.Fprintf(&b, "position = Vector2(%g, %g)\n", cx, cy)
			fmt.Fprintf(&b, "shape = SubResource(\"rect_%d\")\n\n", subStart+i)
		}
	}
	return b.String()
}

type cellRect struct {
	x int
	y int
	w int
	h int
}

// collisionRects merges solid cells into larger rectangles: horizontal runs
// per row, then runs with the same span on consecutive rows.
func collisionRects(m project.GameMap) []cellRect {
	grid := make([][]bool, m.Height)
	for y := range grid {
		grid[y] = make([]bool, m.Width)
	}
	for _, layer := range m.Layers {
		if layer.Type != project.MapCollision {
			continue
		}
		for _, tile := range layer.Tiles {
			if tile.Y >= 0 && tile.Y < m.Height && tile.X >= 0 && tile.X < m.Width {
				grid[tile.Y][tile.X] = true
			}
		}
	}
	type span struct{ x, w int }
	// active maps a span to the index of the rectangle growing downwards.
	active := map[span]int{}
	rects := []cellRect{}
	for y := 0; y < m.Height; y++ {
		row := map[span]bool{}
		x := 0
		for x < m.Width {
			if !grid[y][x] {
				x++
				continue
			}
			start := x
			for x < m.Width && grid[y][x] {
				x++
			}
			key := span{start, x - start}
			row[key] = true
			if index, ok := active[key]; ok {
				rects[index].h++
			} else {
				rects = append(rects, cellRect{x: start, y: y, w: key.w, h: 1})
				active[key] = len(rects) - 1
			}
		}
		for key := range active {
			if !row[key] {
				delete(active, key)
			}
		}
	}
	sort.Slice(rects, func(i, j int) bool {
		if rects[i].y != rects[j].y {
			return rects[i].y < rects[j].y
		}
		return rects[i].x < rects[j].x
	})
	return rects
}

func godotBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func godotDict(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("\"%s\": \"%s\"", tscnString(key), tscnString(values[key])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// tscnString escapes a value for a double-quoted .tscn property.
func tscnString(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return replacer.Replace(value)
}

// safeNodeName strips characters Godot forbids in node names.
func safeNodeName(name string) string {
	trimmed := strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range trimmed {
		switch r {
		case '.', ':', '@', '/', '"', '%', '|', '*', '<', '>', '?', '\\':
			b.WriteRune('_')
		default:
			if r < 0x20 {
				b.WriteRune('_')
				continue
			}
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "Node"
	}
	return out
}

func uniqueNodeName(name string, used map[string]int) string {
	count := used[name]
	used[name] = count + 1
	if count == 0 {
		return name
	}
	return fmt.Sprintf("%s %d", name, count+1)
}

func projectGodot(m project.GameMap, sceneName string) string {
	return fmt.Sprintf(`; Indie Studio 生成的最小 Godot 4 项目，可直接用 Godot 打开本文件夹。
config_version=5

[application]

config/name="%s"
run/main_scene="res://%s"
config/features=PackedStringArray("4.3", "GL Compatibility")

[rendering]

renderer/rendering_method="gl_compatibility"
renderer/rendering_method.mobile="gl_compatibility"
`, tscnString(m.Name+"（Indie Studio）"), tscnString(sceneName))
}

func godotReadme(m project.GameMap, sceneName string) string {
	return fmt.Sprintf(`Indie Studio Godot 导出 · %s

打开方式
  1. 直接用 Godot 4.3+ 打开本文件夹（里面带一个最小 project.godot），首次打开会导入图集，运行即可看到地图。
     命令行直接运行前先导入一次：godot --headless --path . --import
  2. 或把本文件夹复制进你自己的 Godot 项目，改 .tscn / .tres 里的 res:// 路径。

文件
  %s        场景：每层一个 TileMapLayer、对象 Marker2D、合并后的碰撞矩形
  %s.tres    TileSet：每个图集一个 AtlasSource，瓦片 y_sort_origin = 瓦片尺寸
  %s.json    地图数据（图层、瓦片、对象、碰撞网格）
  pixelbench_tiles.gd  @tool 脚本：从 JSON 读取瓦片并调用 set_cell
  roof_fade.gd         可选示例：玩家靠近屋顶时淡出（自行挂到屋顶层）

说明
  - 瓦片单元格由脚本在 _ready() 时写入；编辑器里首次打开后若未显示，重新加载场景即可。
  - 翻转 / 旋转使用 TileSetAtlasSource.TRANSFORM_* 标志，与 TMX 导出语义一致。
  - 碰撞按网格合并为 StaticBody2D 的矩形，图层为 1、掩码为 0。
  - 对象是 Marker2D，带 metadata/pb_* 属性；贴图请在 Godot 里按 assetId 关联。
`, m.Name, sceneName, strings.TrimSuffix(sceneName, ".tscn"), strings.TrimSuffix(sceneName, ".tscn"))
}

const tileLoaderScript = `@tool
extends TileMapLayer

## Fills this layer from an Indie Studio generic map JSON (see <map>.json).
## The scene stays portable: no engine-version-specific packed cell blob.
@export_file("*.json") var cells_path := ""
@export var layer_id := ""

func _ready() -> void:
	populate()

func populate() -> void:
	if cells_path.is_empty() or layer_id.is_empty():
		return
	var text := FileAccess.get_file_as_string(cells_path)
	if text.is_empty():
		push_warning("Indie Studio: cannot read %s" % cells_path)
		return
	var parsed = JSON.parse_string(text)
	if typeof(parsed) != TYPE_DICTIONARY:
		push_warning("Indie Studio: invalid map JSON in %s" % cells_path)
		return
	var columns := {}
	for index in parsed.get("tilesets", []).size():
		var entry = parsed["tilesets"][index]
		columns[index] = max(int(entry.get("columns", 1)), 1)
	clear()
	for layer in parsed.get("layers", []):
		if String(layer.get("id", "")) != layer_id:
			continue
		for cell in layer.get("tiles", []):
			var set_index := int(cell.get("set", 0))
			var cols: int = columns.get(set_index, 1)
			var tile := int(cell.get("tile", 0))
			var atlas := Vector2i(tile % cols, tile / cols)
			var alternate := 0
			var transform := 0
			if bool(cell.get("flipH", false)):
				transform |= TileSetAtlasSource.TRANSFORM_FLIP_H
			if bool(cell.get("flipV", false)):
				transform |= TileSetAtlasSource.TRANSFORM_FLIP_V
			var rotate := int(cell.get("rotate", 0))
			if rotate == 90:
				transform |= TileSetAtlasSource.TRANSFORM_TRANSPOSE | TileSetAtlasSource.TRANSFORM_FLIP_H
			elif rotate == 180:
				transform |= TileSetAtlasSource.TRANSFORM_FLIP_H | TileSetAtlasSource.TRANSFORM_FLIP_V
			elif rotate == 270:
				transform |= TileSetAtlasSource.TRANSFORM_TRANSPOSE | TileSetAtlasSource.TRANSFORM_FLIP_V
			set_cell(Vector2i(int(cell.get("x", 0)), int(cell.get("y", 0))), set_index, atlas, alternate | transform)
		break
`

const roofFadeScript = `@tool
extends TileMapLayer

## Optional helper: fades this roof layer when a player node is close.
## Attach to the roof TileMapLayer and set player_path to your player node.
@export var player_path: NodePath
@export var fade_alpha := 0.32
@export var distance := 128.0

func _process(_delta: float) -> void:
	if Engine.is_editor_hint():
		return
	var player := get_node_or_null(player_path)
	if player == null:
		return
	var target := fade_alpha if global_position.distance_to(player.global_position) < distance else 1.0
	modulate.a = move_toward(modulate.a, target, 0.08)
`
