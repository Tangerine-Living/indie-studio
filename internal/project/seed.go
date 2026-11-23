package project

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
)

var grassColors = []color.NRGBA{
	{0x3e, 0x6b, 0x5a, 0xff}, // deep shade
	{0x4f, 0x8f, 0x4a, 0xff}, // base
	{0x6f, 0xae, 0x5f, 0xff}, // light
	{0x8c, 0xcf, 0x6f, 0xff}, // highlight
	{0xf2, 0xe6, 0xc9, 0xff}, // cream flower
	{0xf2, 0xb8, 0x4b, 0xff}, // gold flower
}

// seedAsset writes a small procedural grass tile so a new project immediately
// has something to look at in the asset library.
func seedAsset(root, paletteID, questID string) (Asset, error) {
	id := NewID()
	dir := assetRoot(root, id)
	if err := os.MkdirAll(filepath.Join(dir, "source"), 0o755); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法创建素材目录", err)
	}
	tile := renderGrassTile(32)
	var tileBuf bytes.Buffer
	if err := png.Encode(&tileBuf, tile); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法生成占位图", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "source", "grass.png"), tileBuf.Bytes(), 0o644); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法写入占位图", err)
	}

	thumb := upscaleNearest(tile, 4)
	var thumbBuf bytes.Buffer
	if err := png.Encode(&thumbBuf, thumb); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法生成缩略图", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "thumb.png"), thumbBuf.Bytes(), 0o644); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法写入缩略图", err)
	}

	now := time.Now().UTC()
	asset := Asset{
		SchemaVersion:  SchemaVersion,
		ID:             id,
		Name:           "草地占位图",
		Type:           AssetTile,
		Tags:           []string{"占位", "草地"},
		Status:         AssetApproved,
		Dimensions:     Dimensions{W: 32, H: 32},
		Pivot:          Point{X: 16, Y: 16},
		PaletteID:      paletteID,
		QuestIDs:       []string{questID},
		License:        "original",
		Source:         AssetSource{Kind: "procedural", Provider: "pixelbench"},
		CurrentVersion: "",
		Versions:       []AssetVersion{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := fsutil.WriteJSONAtomic(assetManifest(root, id), asset, 0o644); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法写入素材清单", err)
	}
	return asset, nil
}

// seedTileAtlas writes a 4×4 demo terrain atlas so a new project can paint a
// map immediately, without importing art first.
func seedTileAtlas(root, paletteID, questID string) (Asset, error) {
	id := NewID()
	dir := assetRoot(root, id)
	if err := os.MkdirAll(filepath.Join(dir, "source"), 0o755); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法创建瓦片集目录", err)
	}
	atlas := renderTileAtlas(32)
	var atlasBuf bytes.Buffer
	if err := png.Encode(&atlasBuf, atlas); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法生成瓦片集", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "source", "atlas.png"), atlasBuf.Bytes(), 0o644); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法写入瓦片集", err)
	}
	thumb := upscaleNearest(atlas, 2)
	var thumbBuf bytes.Buffer
	if err := png.Encode(&thumbBuf, thumb); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法生成瓦片集缩略图", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "thumb.png"), thumbBuf.Bytes(), 0o644); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法写入瓦片集缩略图", err)
	}
	now := time.Now().UTC()
	asset := Asset{
		SchemaVersion: SchemaVersion,
		ID:            id,
		Name:          "小镇瓦片图集",
		Type:          AssetTile,
		Tags:          []string{"瓦片", "示例"},
		Status:        AssetApproved,
		Dimensions:    Dimensions{W: 128, H: 128},
		Pivot:         Point{X: 64, Y: 64},
		PaletteID:     paletteID,
		QuestIDs:      []string{questID},
		License:       "original",
		Source:        AssetSource{Kind: "procedural", Provider: "pixelbench"},
		Versions:      []AssetVersion{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := fsutil.WriteJSONAtomic(assetManifest(root, id), asset, 0o644); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法写入瓦片集清单", err)
	}
	return asset, nil
}

func renderTileAtlas(tile int) *image.NRGBA {
	const columns = 4
	img := image.NewNRGBA(image.Rect(0, 0, tile*columns, tile*columns))
	grass := renderGrassTile(tile)
	drawWithin(img, 0, 0, tile, func(x, y int, set func(color.NRGBA)) {
		set(grass.NRGBAAt(x, y))
	})

	base := func(c color.NRGBA) func(x, y int, set func(color.NRGBA)) {
		return func(_ int, _ int, set func(color.NRGBA)) { set(c) }
	}
	speckle := func(base, dark, light color.NRGBA, density int) func(x, y int, set func(color.NRGBA)) {
		return func(x, y int, set func(color.NRGBA)) {
			c := base
			if (x*7+y*13)%19 == 0 {
				c = dark
			} else if (x*5+y*11)%23 == 0 {
				c = light
			}
			set(c)
		}
	}

	// Row 0: grass variants, path, cobble.
	drawWithin(img, 1, 0, tile, func(x, y int, set func(color.NRGBA)) {
		c := grass.NRGBAAt(x, y)
		if (x == 8 && y == 8) || (x == 9 && y == 8) || (x == 8 && y == 9) {
			c = grassColors[4]
		}
		if x == 22 && y == 20 {
			c = grassColors[5]
		}
		set(c)
	})
	drawWithin(img, 2, 0, tile, speckle(color.NRGBA{0xd9, 0x8f, 0x4a, 0xff}, color.NRGBA{0xb8, 0x6a, 0x36, 0xff}, color.NRGBA{0xf2, 0xc8, 0x86, 0xff}, 3))
	drawWithin(img, 3, 0, tile, func(x, y int, set func(color.NRGBA)) {
		c := color.NRGBA{0x8b, 0x99, 0xab, 0xff}
		if y%8 == 0 || (x+((y/8)%2)*8)%16 == 0 {
			c = color.NRGBA{0x5f, 0x6d, 0x7e, 0xff}
		}
		set(c)
	})

	// Row 1: water, sand, stone wall, roof.
	drawWithin(img, 0, 1, tile, func(x, y int, set func(color.NRGBA)) {
		c := color.NRGBA{0x3e, 0x6b, 0x8a, 0xff}
		if y%9 == (x/7)%9 {
			c = color.NRGBA{0x6e, 0xa8, 0xc8, 0xff}
		}
		set(c)
	})
	drawWithin(img, 1, 1, tile, speckle(color.NRGBA{0xe8, 0xc0, 0x7a, 0xff}, color.NRGBA{0xc9, 0xa0, 0x5a, 0xff}, color.NRGBA{0xf7, 0xe3, 0xb0, 0xff}, 4))
	drawWithin(img, 2, 1, tile, func(x, y int, set func(color.NRGBA)) {
		c := color.NRGBA{0x7a, 0x7f, 0x86, 0xff}
		if y%8 == 0 || (x+((y/8)%2)*8)%16 == 0 {
			c = color.NRGBA{0x4d, 0x52, 0x5a, 0xff}
		}
		set(c)
	})
	drawWithin(img, 3, 1, tile, func(x, y int, set func(color.NRGBA)) {
		c := color.NRGBA{0xb0, 0x4e, 0x3e, 0xff}
		if y%6 == 0 {
			c = color.NRGBA{0x8a, 0x38, 0x2e, 0xff}
		} else if (x+y/6*4)%9 == 0 {
			c = color.NRGBA{0xc9, 0x6a, 0x4e, 0xff}
		}
		set(c)
	})

	// Row 2: blue roof, wood wall, door, tree.
	drawWithin(img, 0, 2, tile, func(x, y int, set func(color.NRGBA)) {
		c := color.NRGBA{0x3e, 0x5a, 0x7a, 0xff}
		if y%6 == 0 {
			c = color.NRGBA{0x2c, 0x42, 0x5c, 0xff}
		}
		set(c)
	})
	drawWithin(img, 1, 2, tile, func(x, y int, set func(color.NRGBA)) {
		c := color.NRGBA{0x8a, 0x5a, 0x36, 0xff}
		if x%8 == 0 {
			c = color.NRGBA{0x63, 0x3e, 0x24, 0xff}
		} else if y%7 == 0 {
			c = color.NRGBA{0xa6, 0x72, 0x48, 0xff}
		}
		set(c)
	})
	drawWithin(img, 2, 2, tile, func(x, y int, set func(color.NRGBA)) {
		c := color.NRGBA{0x5c, 0x3a, 0x24, 0xff}
		if x < 3 || x > tile-4 || y < 3 {
			c = color.NRGBA{0x8a, 0x5a, 0x36, 0xff}
		}
		if x >= tile/2+3 && x <= tile/2+4 && y >= tile/2 && y <= tile/2+2 {
			c = color.NRGBA{0xf2, 0xd8, 0x78, 0xff}
		}
		set(c)
	})
	drawWithin(img, 3, 2, tile, func(x, y int, set func(color.NRGBA)) {
		c := grass.NRGBAAt(x, y)
		if x >= 13 && x <= 18 && y >= 20 {
			c = color.NRGBA{0x78, 0x54, 0x3a, 0xff}
		}
		if (x-16)*(x-16)+(y-12)*(y-12) <= 100 {
			c = color.NRGBA{0x2f, 0x5b, 0x3e, 0xff}
			if (x-12)*(x-12)+(y-9)*(y-9) <= 30 {
				c = color.NRGBA{0x4f, 0x8f, 0x4a, 0xff}
			}
		}
		set(c)
	})

	// Row 3: bush, flower bed, fence, void.
	drawWithin(img, 0, 3, tile, func(x, y int, set func(color.NRGBA)) {
		c := grass.NRGBAAt(x, y)
		if (x-16)*(x-16)+(y-18)*(y-18) <= 90 {
			c = color.NRGBA{0x3e, 0x6b, 0x4a, 0xff}
			if x%4 == 0 && y%4 == 0 {
				c = color.NRGBA{0x6f, 0xae, 0x5f, 0xff}
			}
		}
		set(c)
	})
	drawWithin(img, 1, 3, tile, func(x, y int, set func(color.NRGBA)) {
		c := grass.NRGBAAt(x, y)
		if (x%9 == 3 && y%9 == 4) || (x%9 == 4 && y%9 == 4) || (x%9 == 3 && y%9 == 5) {
			c = color.NRGBA{0xf2, 0xb8, 0x4b, 0xff}
		}
		set(c)
	})
	drawWithin(img, 2, 3, tile, func(x, y int, set func(color.NRGBA)) {
		c := grass.NRGBAAt(x, y)
		if x%10 == 4 || (y >= 12 && y <= 14) {
			c = color.NRGBA{0x8a, 0x5a, 0x36, 0xff}
		}
		set(c)
	})
	drawWithin(img, 3, 3, tile, base(color.NRGBA{0x0f, 0x1b, 0x2a, 0xff}))
	return img
}

// drawWithin paints one atlas cell. The callback receives cell-local pixels.
func drawWithin(atlas *image.NRGBA, col, row, tile int, paint func(x, y int, set func(color.NRGBA))) {
	set := func(c color.NRGBA) {}
	x, y := 0, 0
	set = func(c color.NRGBA) {
		atlas.SetNRGBA(col*tile+x, row*tile+y, c)
	}
	for y = 0; y < tile; y++ {
		for x = 0; x < tile; x++ {
			paint(x, y, set)
		}
	}
}

// seedTownMap writes a playable example plaza referencing the demo atlas.
func seedTownMap(root string, project Project, atlas Asset) (GameMap, error) {
	const width, height = 40, 30
	tiles := func(cells ...MapTile) []MapTile { return cells }
	ground := make([]MapTile, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ground = append(ground, MapTile{X: x, Y: y, Set: 0, Tile: 0})
		}
	}
	ground = append(ground,
		MapTile{X: 1, Y: 1, Set: 0, Tile: 1},
		MapTile{X: 30, Y: 4, Set: 0, Tile: 1},
		MapTile{X: 34, Y: 22, Set: 0, Tile: 1},
	)
	replaceGround := func(x0, y0, x1, y1, tileID int) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				for i := range ground {
					if ground[i].X == x && ground[i].Y == y {
						ground[i].Tile = tileID
						break
					}
				}
			}
		}
	}
	// Cobble plaza, a crossroad and a pond with a sandy rim.
	replaceGround(12, 9, 27, 20, 3)
	replaceGround(19, 0, 20, 29, 2)
	replaceGround(0, 15, 39, 16, 2)
	replaceGround(3, 3, 9, 8, 5)
	replaceGround(4, 4, 8, 7, 4)

	decoration := []MapTile{}
	for _, tree := range [][2]int{{1, 3}, {2, 20}, {36, 6}, {37, 25}, {10, 26}, {30, 27}} {
		decoration = append(decoration, MapTile{X: tree[0], Y: tree[1], Set: 0, Tile: 11})
	}
	for _, bush := range [][2]int{{14, 7}, {25, 6}, {11, 22}, {28, 22}, {33, 12}} {
		decoration = append(decoration, MapTile{X: bush[0], Y: bush[1], Set: 0, Tile: 12})
	}
	for _, flowers := range [][2]int{{11, 10}, {28, 10}, {11, 19}, {28, 19}, {16, 6}, {23, 23}} {
		decoration = append(decoration, MapTile{X: flowers[0], Y: flowers[1], Set: 0, Tile: 13})
	}

	building := []MapTile{}
	roof := []MapTile{}
	for x := 23; x <= 28; x++ {
		roof = append(roof, MapTile{X: x, Y: 8, Set: 0, Tile: 7})
		roof = append(roof, MapTile{X: x, Y: 9, Set: 0, Tile: 7})
		building = append(building, MapTile{X: x, Y: 10, Set: 0, Tile: 9})
		building = append(building, MapTile{X: x, Y: 11, Set: 0, Tile: 9})
	}
	building = append(building, MapTile{X: 25, Y: 11, Set: 0, Tile: 10})

	collision := []MapTile{}
	for y := 3; y <= 8; y++ {
		for x := 3; x <= 9; x++ {
			collision = append(collision, MapTile{X: x, Y: y})
		}
	}
	for y := 8; y <= 11; y++ {
		for x := 23; x <= 28; x++ {
			collision = append(collision, MapTile{X: x, Y: y})
		}
	}

	m := GameMap{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Name:          "小镇广场",
		TileSize:      project.Settings.TileSize,
		Width:         width,
		Height:        height,
		Tilesets: []MapTileset{{
			ID:        "town-atlas",
			AssetID:   atlas.ID,
			TileSize:  project.Settings.TileSize,
			Columns:   4,
			TileCount: 16,
		}},
		Layers: []MapLayer{
			{ID: "ground", Name: "地面", Type: MapGround, Visible: true, Opacity: 1, Tiles: ground},
			{ID: "decoration", Name: "装饰", Type: MapDecoration, Visible: true, Opacity: 1, Tiles: decoration},
			{ID: "building", Name: "建筑", Type: MapBuilding, Visible: true, Opacity: 1, Tiles: building},
			{ID: "roof", Name: "屋顶", Type: MapRoof, Visible: true, Opacity: 1, Tiles: roof},
			{ID: "objects", Name: "对象", Type: MapLayerObject, Visible: true, Opacity: 1, Tiles: tiles()},
			{ID: "collision", Name: "碰撞", Type: MapCollision, Visible: true, Opacity: 0.5, Tiles: collision},
		},
		Objects: []MapObject{
			{ID: NewID(), Name: "玩家出生点", Kind: MapObjectSpawn, X: 19.5, Y: 16.5, W: 1, H: 1, YSort: true, Color: "#6ec6e8"},
			{ID: NewID(), Name: "面包师", Kind: MapObjectNPC, X: 16, Y: 12.5, W: 1, H: 1, YSort: true, Solid: true, Color: "#f2b84b", Properties: map[string]string{"dialog": "今天的面包刚出炉。"}},
			{ID: NewID(), Name: "宝箱", Kind: MapObjectChest, X: 26, Y: 18, W: 1, H: 1, YSort: true, Solid: true, Color: "#d98f4a", Properties: map[string]string{"item": "铜币"}},
			{ID: NewID(), Name: "广场入口", Kind: MapObjectTrigger, X: 19, Y: 28, W: 2, H: 1, YSort: false, Color: "#8ccf6f", Properties: map[string]string{"event": "enter-square"}},
		},
		Settings: MapSettings{YSort: true, RoofFade: true, HeightStep: 16, Ambient: "#8090a0"},
	}
	if err := ValidateMap(&m); err != nil {
		return GameMap{}, err
	}
	if err := fsutil.WriteJSONAtomic(MapPath(root, m.ID), m, 0o644); err != nil {
		return GameMap{}, apperr.Wrap("IO", "无法写入示例地图", err)
	}
	return m, nil
}

func renderGrassTile(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	base := grassColors[1]
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, base)
		}
	}

	rng := rand.New(rand.NewSource(20261121))
	for i := 0; i < 70; i++ {
		x := rng.Intn(size - 2)
		y := rng.Intn(size - 2)
		c := grassColors[2]
		if rng.Intn(3) == 0 {
			c = grassColors[0]
		}
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				img.SetNRGBA(x+dx, y+dy, c)
			}
		}
	}
	for _, p := range []image.Point{{7, 9}, {24, 6}, {18, 25}} {
		img.SetNRGBA(p.X, p.Y, grassColors[4])
		img.SetNRGBA(p.X+1, p.Y, grassColors[5])
		img.SetNRGBA(p.X, p.Y+1, grassColors[5])
	}
	// Slightly darker edge keeps a tile grid readable when repeated.
	edge := grassColors[0]
	for i := 0; i < size; i++ {
		img.SetNRGBA(i, size-1, edge)
		img.SetNRGBA(size-1, i, edge)
	}
	return img
}

func upscaleNearest(src *image.NRGBA, scale int) *image.NRGBA {
	w := src.Bounds().Dx() * scale
	h := src.Bounds().Dy() * scale
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetNRGBA(x, y, src.NRGBAAt(x/scale, y/scale))
		}
	}
	return dst
}
