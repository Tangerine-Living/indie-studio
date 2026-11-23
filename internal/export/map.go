// Package export renders project data into engine-ready files. Map export
// covers Tiled TMX/TSX (CSV data) and a generic JSON sidecar that carries the
// collision grid and Y-sort metadata.
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// MapExportResult is the folder and files produced by ExportMap.
type MapExportResult struct {
	Dir    string   `json:"dir"`
	Files  []string `json:"files"`
	Format string   `json:"format"` // tiled | godot | all
}

type tilesetInfo struct {
	firstGID uint32
	asset    project.Asset
	image    string
	tsx      string
}

// TMX flip flags. Rotation is expressed as a combination of the diagonal and
// horizontal/vertical flags, matching Tiled's global tile IDs.
const (
	tmxFlipH = uint32(0x80000000)
	tmxFlipV = uint32(0x40000000)
	tmxFlipD = uint32(0x20000000)
)

// ExportMap writes Tiled and generic files under exports/maps/<name>/.
// Format also accepts "godot" or "all"; those prefixes export into the same
// exports/maps/ tree (see ExportGodot).
func ExportMap(store *project.Store, mapID, format string) (MapExportResult, error) {
	switch format {
	case "", "tiled":
		return exportTiled(store, mapID)
	case "godot":
		return ExportGodot(store, mapID)
	case "all":
		tiled, err := exportTiled(store, mapID)
		if err != nil {
			return MapExportResult{}, err
		}
		godot, err := ExportGodot(store, mapID)
		if err != nil {
			return MapExportResult{}, err
		}
		tiled.Files = append(tiled.Files, godot.Files...)
		tiled.Format = "all"
		return tiled, nil
	default:
		return MapExportResult{}, apperr.New("EXPORT_FORMAT", "未知的导出格式").WithDetail(format)
	}
}

func exportTiled(store *project.Store, mapID string) (MapExportResult, error) {
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

	dir := filepath.Join(project.ExportsDir(store.Root()), "maps", safeName(m.Name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return MapExportResult{}, apperr.Wrap("IO", "无法创建地图导出目录", err)
	}
	result := MapExportResult{Dir: dir, Files: []string{}, Format: "tiled"}
	write := func(name string, data []byte) error {
		if err := fsutil.WriteFileAtomic(filepath.Join(dir, name), data, 0o644); err != nil {
			return apperr.Wrap("IO", "无法写入导出文件", err).WithDetail(name)
		}
		result.Files = append(result.Files, name)
		return nil
	}

	infos := make([]tilesetInfo, len(m.Tilesets))
	firstGID := uint32(1)
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
		tsxName := fmt.Sprintf("tileset-%d-%s.tsx", i+1, safeName(asset.Name))
		infos[i] = tilesetInfo{
			firstGID: firstGID,
			asset:    asset,
			image:    imageName,
			tsx:      tsxName,
		}
		firstGID += uint32(set.TileCount)
		if err := write(tsxName, []byte(tilesetXML(set, asset, imageName))); err != nil {
			return MapExportResult{}, err
		}
	}

	tmx := tmxXML(m, infos)
	if err := write(safeName(m.Name)+".tmx", []byte(tmx)); err != nil {
		return MapExportResult{}, err
	}
	if err := write(safeName(m.Name)+".json", []byte(genericJSON(m))); err != nil {
		return MapExportResult{}, err
	}
	return result, nil
}

// assetImagePath prefers the current version, then the original import, then
// the thumbnail. This mirrors GetAssetSource in app.go without the HTTP layer.
func assetImagePath(store *project.Store, assetID string) (string, error) {
	asset, err := store.GetAsset(assetID)
	if err != nil {
		return "", err
	}
	base := project.AssetDir(store.Root(), assetID)
	if asset.CurrentVersion != "" {
		for _, name := range []string{"output.png", "sheet.png"} {
			candidate := filepath.Join(base, "versions", asset.CurrentVersion, name)
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	if entries, readErr := os.ReadDir(filepath.Join(base, "source")); readErr == nil {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			switch strings.ToLower(filepath.Ext(entry.Name())) {
			case ".png", ".jpg", ".jpeg", ".webp":
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		if len(names) > 0 {
			return filepath.Join(base, "source", names[0]), nil
		}
	}
	thumb := filepath.Join(base, "thumb.png")
	if _, statErr := os.Stat(thumb); statErr == nil {
		return thumb, nil
	}
	return "", apperr.New("MAP_TILESET_MISSING", "瓦片集素材没有可用的图片").WithDetail(assetID)
}

func copyInto(dir, src, name string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return apperr.Wrap("IO", "无法读取瓦片集图片", err).WithDetail(src)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, name), data, 0o644); err != nil {
		return apperr.Wrap("IO", "无法写入瓦片集图片", err).WithDetail(name)
	}
	return nil
}

func tilesetXML(set project.MapTileset, asset project.Asset, image string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<tileset version="1.10" tiledversion="1.11.0" name="%s" tilewidth="%d" tileheight="%d" tilecount="%d" columns="%d">`+"\n",
		xmlAttr(asset.Name), set.TileSize, set.TileSize, set.TileCount, set.Columns)
	fmt.Fprintf(&b, ` <image source="%s" width="%d" height="%d"/>`+"\n",
		xmlAttr(image), asset.Dimensions.W, asset.Dimensions.H)
	b.WriteString("</tileset>\n")
	return b.String()
}

func tmxXML(m project.GameMap, infos []tilesetInfo) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<map version="1.10" tiledversion="1.11.0" orientation="orthogonal" renderorder="right-down" width="%d" height="%d" tilewidth="%d" tileheight="%d" infinite="0" nextlayerid="%d" nextobjectid="%d">`+"\n",
		m.Width, m.Height, m.TileSize, m.TileSize, len(m.Layers)+1, len(m.Objects)+1)
	writeProperties(&b, "  ", mapProperties(m))
	for _, info := range infos {
		fmt.Fprintf(&b, ` <tileset firstgid="%d" source="%s"/>`+"\n", info.firstGID, xmlAttr(info.tsx))
	}
	for index, layer := range m.Layers {
		if layer.Type == project.MapLayerObject {
			continue
		}
		id := index + 1
		fmt.Fprintf(&b, ` <layer id="%d" name="%s" width="%d" height="%d" opacity="%g"%s>`+"\n",
			id, xmlAttr(layer.Name), m.Width, m.Height, clampOpacity(layer.Opacity), visibility(layer.Visible))
		fmt.Fprintf(&b, `  <data encoding="csv">%s</data>`+"\n", tmxCSV(m, layer, infos))
		b.WriteString("  <properties>\n")
		fmt.Fprintf(&b, `   <property name="type" value="%s"/>`+"\n", xmlAttr(layer.Type))
		fmt.Fprintf(&b, `   <property name="elevation" type="int" value="%d"/>`+"\n", layer.Elevation)
		if layer.Type == project.MapCollision {
			b.WriteString(`   <property name="collision" type="bool" value="true"/>` + "\n")
		}
		b.WriteString("  </properties>\n")
		b.WriteString(" </layer>\n")
	}
	fmt.Fprintf(&b, ` <objectgroup id="%d" name="objects">`+"\n", len(m.Layers)+1)
	for index, obj := range m.Objects {
		fmt.Fprintf(&b, `  <object id="%d" name="%s" x="%g" y="%g" width="%g" height="%g">`+"\n",
			index+1, xmlAttr(obj.Name), obj.X*float64(m.TileSize), obj.Y*float64(m.TileSize),
			obj.W*float64(m.TileSize), obj.H*float64(m.TileSize))
		props := [][3]string{
			{"kind", "string", obj.Kind},
			{"elevation", "int", strconv.Itoa(obj.Elevation)},
			{"ySort", "bool", strconv.FormatBool(obj.YSort)},
			{"roofFade", "bool", strconv.FormatBool(obj.RoofFade)},
			{"solid", "bool", strconv.FormatBool(obj.Solid)},
			{"depth", "int", strconv.Itoa(objectDepth(m, obj))},
		}
		if obj.AssetID != "" {
			props = append(props, [3]string{"assetId", "string", obj.AssetID})
		}
		newProps := make([][3]string, 0, len(props)+len(obj.Properties))
		newProps = append(newProps, props...)
		keys := make([]string, 0, len(obj.Properties))
		for key := range obj.Properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			newProps = append(newProps, [3]string{key, "string", obj.Properties[key]})
		}
		b.WriteString("   <properties>\n")
		for _, prop := range newProps {
			fmt.Fprintf(&b, `    <property name="%s" type="%s" value="%s"/>`+"\n",
				xmlAttr(prop[0]), prop[1], xmlAttr(prop[2]))
		}
		b.WriteString("   </properties>\n")
		b.WriteString("  </object>\n")
	}
	b.WriteString(" </objectgroup>\n")
	b.WriteString("</map>\n")
	return b.String()
}

func tmxCSV(m project.GameMap, layer project.MapLayer, infos []tilesetInfo) string {
	cells := make(map[int]project.MapTile, len(layer.Tiles))
	for _, tile := range layer.Tiles {
		cells[tile.Y*m.Width+tile.X] = tile
	}
	var b strings.Builder
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			if x > 0 {
				b.WriteByte(',')
			}
			tile, ok := cells[y*m.Width+x]
			if !ok || layer.Type == project.MapCollision {
				if layer.Type == project.MapCollision && ok {
					// Collision cells reuse the first tile as a marker; the layer
					// property tells importers how to read it.
					b.WriteString(strconv.Itoa(int(infosGID(infos, 0))))
				} else {
					b.WriteByte('0')
				}
				continue
			}
			if tile.Set < 0 || tile.Set >= len(infos) {
				b.WriteByte('0')
				continue
			}
			gid := infosGID(infos, tile.Set) + uint32(tile.Tile)
			gid |= flipFlags(tile)
			b.WriteString(strconv.FormatUint(uint64(gid), 10))
		}
		if y < m.Height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func infosGID(infos []tilesetInfo, set int) uint32 {
	if set < 0 || set >= len(infos) {
		return 0
	}
	return infos[set].firstGID
}

func flipFlags(tile project.MapTile) uint32 {
	var flags uint32
	if tile.FlipH {
		flags |= tmxFlipH
	}
	if tile.FlipV {
		flags |= tmxFlipV
	}
	switch tile.Rotate {
	case 90:
		flags |= tmxFlipH | tmxFlipD
	case 180:
		flags |= tmxFlipH | tmxFlipV
	case 270:
		flags |= tmxFlipV | tmxFlipD
	}
	return flags
}

func mapProperties(m project.GameMap) [][3]string {
	return [][3]string{
		{"pixelbench.schemaVersion", "int", strconv.Itoa(m.SchemaVersion)},
		{"tileSize", "int", strconv.Itoa(m.TileSize)},
		{"ySort", "bool", strconv.FormatBool(m.Settings.YSort)},
		{"roofFade", "bool", strconv.FormatBool(m.Settings.RoofFade)},
		{"heightStep", "int", strconv.Itoa(m.Settings.HeightStep)},
		{"ambient", "string", m.Settings.Ambient},
	}
}

func writeProperties(b *strings.Builder, indent string, props [][3]string) {
	b.WriteString(indent + "<properties>\n")
	for _, prop := range props {
		fmt.Fprintf(b, "%s <property name=\"%s\" type=\"%s\" value=\"%s\"/>\n",
			indent, xmlAttr(prop[0]), prop[1], xmlAttr(prop[2]))
	}
	b.WriteString(indent + "</properties>\n")
}

func visibility(visible bool) string {
	if visible {
		return ""
	}
	return ` visible="0"`
}

func clampOpacity(value float64) float64 {
	if value <= 0 || value > 1 {
		return 1
	}
	return value
}

func objectDepth(m project.GameMap, obj project.MapObject) int {
	return int((obj.Y+obj.H)*float64(m.TileSize)) + obj.Elevation*m.Settings.HeightStep
}

func genericJSON(m project.GameMap) []byte {
	type objectOut struct {
		project.MapObject
		Depth int `json:"depth"`
	}
	objects := make([]objectOut, 0, len(m.Objects))
	for _, obj := range m.Objects {
		objects = append(objects, objectOut{MapObject: obj, Depth: objectDepth(m, obj)})
	}
	collision := make([][]int, m.Height)
	for y := range collision {
		collision[y] = make([]int, m.Width)
	}
	for _, layer := range m.Layers {
		if layer.Type != project.MapCollision {
			continue
		}
		for _, tile := range layer.Tiles {
			if tile.Y >= 0 && tile.Y < m.Height && tile.X >= 0 && tile.X < m.Width {
				collision[tile.Y][tile.X] = 1
			}
		}
	}
	payload := struct {
		Format    string               `json:"format"`
		Version   int                  `json:"version"`
		Exported  string               `json:"exportedAt"`
		MapID     string               `json:"mapId"`
		Name      string               `json:"name"`
		TileSize  int                  `json:"tileSize"`
		Width     int                  `json:"width"`
		Height    int                  `json:"height"`
		Tilesets  []project.MapTileset `json:"tilesets"`
		Layers    []project.MapLayer   `json:"layers"`
		Collision [][]int              `json:"collision"`
		Objects   []objectOut          `json:"objects"`
		Settings  project.MapSettings  `json:"settings"`
	}{
		Format:    "pixelbench-map",
		Version:   1,
		Exported:  time.Now().UTC().Format(time.RFC3339),
		MapID:     m.ID,
		Name:      m.Name,
		TileSize:  m.TileSize,
		Width:     m.Width,
		Height:    m.Height,
		Tilesets:  m.Tilesets,
		Layers:    m.Layers,
		Collision: collision,
		Objects:   objects,
		Settings:  m.Settings,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return []byte("{}")
	}
	return append(data, '\n')
}

// safeName keeps Chinese characters but strips Windows-reserved characters.
func safeName(name string) string {
	trimmed := strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range trimmed {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			b.WriteRune('_')
		default:
			if r < 0x20 {
				b.WriteRune('_')
				continue
			}
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), " .")
	runes := []rune(out)
	if len(runes) > 64 {
		out = string(runes[:64])
	}
	if out == "" || out == "." || out == ".." {
		return "map"
	}
	upper := strings.ToUpper(out)
	for _, reserved := range []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"} {
		if upper == reserved {
			return "_" + out
		}
	}
	return out
}

func xmlAttr(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
		"\n", "&#10;",
	)
	return replacer.Replace(value)
}
