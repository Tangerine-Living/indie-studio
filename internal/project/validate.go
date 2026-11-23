package project

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"pixelbench/internal/apperr"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func isTileSize(n int) bool {
	return n == 16 || n == 24 || n == 32 || n == 48
}

func choice(value string, allowed []string, fallback string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback, nil
	}
	if !slices.Contains(allowed, value) {
		return "", apperr.Newf("INVALID_VALUE", "无效的取值：%s", value)
	}
	return value, nil
}

// ValidateProject normalises defaults and validates an on-disk project.
func ValidateProject(p *Project) error {
	if p == nil {
		return apperr.New("INVALID_PROJECT", "项目数据为空")
	}
	if _, err := SanitizeProjectName(p.Name); err != nil {
		return err
	}
	if p.SchemaVersion != SchemaVersion {
		return apperr.Newf("SCHEMA_MISMATCH", "项目 schema 版本不匹配（%d）", p.SchemaVersion)
	}
	if p.ID != "" && !ValidID(p.ID) {
		return apperr.New("INVALID_PROJECT", "项目 ID 不合法")
	}
	if p.ID == "" {
		p.ID = NewID()
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	engine, err := choice(p.Engine, []string{"godot", "tiled", "generic"}, "godot")
	if err != nil {
		return err
	}
	p.Engine = engine
	stage, err := choice(p.Stage, []string{StageConcept, StagePrototype, StageProduction, StagePolish, StageRelease}, StageConcept)
	if err != nil {
		return err
	}
	p.Stage = stage
	p.CoverAssetID = strings.TrimSpace(p.CoverAssetID)
	if p.CoverAssetID != "" && !ValidID(p.CoverAssetID) {
		p.CoverAssetID = ""
	}

	if p.Settings.TileSize == 0 {
		p.Settings.TileSize = 32
	}
	if !isTileSize(p.Settings.TileSize) {
		return apperr.New("INVALID_TILE_SIZE", "瓦片尺寸必须是 16、24、32 或 48")
	}
	if p.Settings.PixelScale <= 0 {
		p.Settings.PixelScale = 4
	}
	if p.Settings.UILanguage == "" {
		p.Settings.UILanguage = "zh-CN"
	}
	if p.Settings.DefaultPaletteID == "" {
		p.Settings.DefaultPaletteID = "default"
	}
	if p.Settings.StyleGuide.Perspective == "" {
		p.Settings.StyleGuide = defaultStyleGuide()
	}
	if p.Settings.StyleGuide.MaxColors < 2 || p.Settings.StyleGuide.MaxColors > 64 {
		p.Settings.StyleGuide.MaxColors = 16
	}
	if len(p.Members) == 0 {
		p.Members = []Member{{ID: "m1", Name: "我", Role: "独立开发者"}}
	}
	if p.Milestones == nil {
		p.Milestones = []Milestone{}
	}
	return nil
}

// ValidateQuest normalises defaults and validates a quest.
func ValidateQuest(q *Quest) error {
	if q == nil {
		return apperr.New("INVALID_QUEST", "任务数据为空")
	}
	q.Title = strings.TrimSpace(q.Title)
	if q.Title == "" {
		return apperr.New("INVALID_QUEST", "任务标题不能为空")
	}
	if len([]rune(q.Title)) > 200 {
		return apperr.New("INVALID_QUEST", "任务标题不能超过 200 个字符")
	}
	if q.ID == "" {
		q.ID = NewID()
	}
	if !ValidID(q.ID) {
		return apperr.New("INVALID_QUEST", "任务 ID 不合法")
	}
	var err error
	if q.Type, err = choice(q.Type, []string{QuestTypeAsset, QuestTypeCode, QuestTypeDesign, QuestTypeAudio, QuestTypeMarketing}, QuestTypeAsset); err != nil {
		return err
	}
	if q.Status, err = choice(q.Status, []string{QuestBacklog, QuestTodo, QuestDoing, QuestReview, QuestDone}, QuestTodo); err != nil {
		return err
	}
	if q.Priority, err = choice(q.Priority, []string{PriorityLow, PriorityMedium, PriorityHigh}, PriorityMedium); err != nil {
		return err
	}
	if q.Brief.AssetType == "" {
		q.Brief.AssetType = AssetSprite
	}
	if q.Brief.TileSize == 0 {
		q.Brief.TileSize = 32
	}
	if !isTileSize(q.Brief.TileSize) {
		return apperr.New("INVALID_QUEST", "任务 brief 的瓦片尺寸不合法")
	}
	if q.Brief.PaletteID == "" {
		q.Brief.PaletteID = "default"
	}
	if q.Brief.References == nil {
		q.Brief.References = []string{}
	}
	if q.Acceptance == nil {
		q.Acceptance = []string{}
	}
	if q.Comments == nil {
		q.Comments = []Comment{}
	}
	if q.Links.AssetIDs == nil {
		q.Links.AssetIDs = []string{}
	}
	if q.Links.MapIDs == nil {
		q.Links.MapIDs = []string{}
	}
	if q.Links.URLs == nil {
		q.Links.URLs = []string{}
	}
	q.SchemaVersion = SchemaVersion
	now := time.Now().UTC()
	if q.CreatedAt.IsZero() {
		q.CreatedAt = now
	}
	if q.UpdatedAt.IsZero() {
		q.UpdatedAt = now
	}
	return nil
}

// ValidateAsset normalises defaults and validates an asset manifest.
func ValidateAsset(a *Asset) error {
	if a == nil {
		return apperr.New("INVALID_ASSET", "素材数据为空")
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return apperr.New("INVALID_ASSET", "素材名称不能为空")
	}
	if a.ID == "" {
		a.ID = NewID()
	}
	if !ValidID(a.ID) {
		return apperr.New("INVALID_ASSET", "素材 ID 不合法")
	}
	var err error
	if a.Type, err = choice(a.Type, []string{AssetSprite, AssetBuilding, AssetTile, AssetProp, AssetUI, AssetFX, AssetOther}, AssetSprite); err != nil {
		return err
	}
	if a.Status, err = choice(a.Status, []string{AssetDraft, AssetReview, AssetApproved, AssetDeprecated}, AssetDraft); err != nil {
		return err
	}
	if a.Dimensions.W <= 0 || a.Dimensions.H <= 0 {
		return apperr.New("INVALID_ASSET", "素材尺寸必须大于 0")
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}
	if a.QuestIDs == nil {
		a.QuestIDs = []string{}
	}
	if a.Versions == nil {
		a.Versions = []AssetVersion{}
	}
	if a.PaletteID == "" {
		a.PaletteID = "default"
	}
	if a.License == "" {
		a.License = "original"
	}
	if a.Pivot.X == 0 && a.Pivot.Y == 0 {
		a.Pivot = Point{X: a.Dimensions.W / 2, Y: a.Dimensions.H - 1}
	}
	a.SchemaVersion = SchemaVersion
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = now
	}
	return nil
}

var entityKinds = []string{EntityCharacter, EntityPlace, EntityFaction, EntityItem, EntityEvent, EntityTerm}
var entityStatuses = []string{EntityIdea, EntityDraft, EntityStable}

// ValidateEntity normalises defaults and validates one piece of setting.
func ValidateEntity(e *Entity) error {
	if e == nil {
		return apperr.New("INVALID_ENTITY", "设定数据为空")
	}
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return apperr.New("INVALID_ENTITY", "设定名称不能为空")
	}
	if len([]rune(e.Name)) > 120 {
		return apperr.New("INVALID_ENTITY", "设定名称不能超过 120 个字符")
	}
	if e.ID == "" {
		e.ID = NewID()
	}
	if !ValidID(e.ID) {
		return apperr.New("INVALID_ENTITY", "设定 ID 不合法")
	}
	var err error
	if e.Kind, err = choice(e.Kind, entityKinds, EntityCharacter); err != nil {
		return apperr.New("INVALID_ENTITY", "设定类型不合法").WithDetail(e.Kind)
	}
	if e.Status, err = choice(e.Status, entityStatuses, EntityDraft); err != nil {
		return apperr.New("INVALID_ENTITY", "设定状态不合法").WithDetail(e.Status)
	}
	e.Summary = strings.TrimSpace(e.Summary)
	if len([]rune(e.Summary)) > 300 {
		e.Summary = string([]rune(e.Summary)[:300])
	}
	e.Body = strings.TrimSpace(e.Body)
	if len([]rune(e.Body)) > 100000 {
		return apperr.New("INVALID_ENTITY", "设定正文不能超过 10 万个字符")
	}
	e.Aliases = cleanStrings(e.Aliases, 32, 80)
	e.Tags = cleanStrings(e.Tags, 32, 40)

	fields := make([]EntityField, 0, len(e.Fields))
	seenKeys := map[string]bool{}
	for _, field := range e.Fields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		if len([]rune(key)) > 40 {
			key = string([]rune(key)[:40])
		}
		lower := strings.ToLower(key)
		if seenKeys[lower] {
			continue
		}
		seenKeys[lower] = true
		value := strings.TrimSpace(field.Value)
		if len([]rune(value)) > 2000 {
			value = string([]rune(value)[:2000])
		}
		fields = append(fields, EntityField{Key: key, Value: value})
		if len(fields) >= 100 {
			break
		}
	}
	e.Fields = fields

	relations := make([]EntityRelation, 0, len(e.Relations))
	seenRelations := map[string]bool{}
	for _, relation := range e.Relations {
		target := strings.TrimSpace(relation.EntityID)
		if !ValidID(target) || target == e.ID {
			continue
		}
		label := strings.TrimSpace(relation.Label)
		if len([]rune(label)) > 60 {
			label = string([]rune(label)[:60])
		}
		if label == "" {
			label = "相关"
		}
		key := target + "\x00" + label
		if seenRelations[key] {
			continue
		}
		seenRelations[key] = true
		relations = append(relations, EntityRelation{EntityID: target, Label: label})
		if len(relations) >= 200 {
			break
		}
	}
	e.Relations = relations

	e.AssetIDs = cleanIDs(e.AssetIDs, 200)
	e.MapIDs = cleanIDs(e.MapIDs, 200)
	e.SchemaVersion = SchemaVersion
	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = now
	}
	return nil
}

// cleanStrings trims entries, drops empties and deduplicates case-insensitively.
func cleanStrings(values []string, maxCount, maxLen int) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len([]rune(value)) > maxLen {
			value = string([]rune(value)[:maxLen])
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
		if len(out) >= maxCount {
			break
		}
	}
	return out
}

// cleanIDs keeps valid, unique IDs in their original order.
func cleanIDs(values []string, maxCount int) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !ValidID(value) || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= maxCount {
			break
		}
	}
	return out
}

var mapLayerTypes = []string{MapGround, MapDecoration, MapBuilding, MapRoof, MapLayerObject, MapCollision, MapLight}
var mapObjectKinds = []string{MapObjectSpawn, MapObjectNPC, MapObjectChest, MapObjectTrigger, MapObjectBuilding, MapObjectProp}

// ValidateMap normalises defaults, drops out-of-bounds tiles and validates a map.
func ValidateMap(m *GameMap) error {
	if m == nil {
		return apperr.New("INVALID_MAP", "地图数据为空")
	}
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return apperr.New("INVALID_MAP", "地图名称不能为空")
	}
	if len([]rune(m.Name)) > 100 {
		return apperr.New("INVALID_MAP", "地图名称不能超过 100 个字符")
	}
	if m.ID == "" {
		m.ID = NewID()
	}
	if !ValidID(m.ID) {
		return apperr.New("INVALID_MAP", "地图 ID 不合法")
	}
	if m.TileSize == 0 {
		m.TileSize = 32
	}
	if !isTileSize(m.TileSize) {
		return apperr.New("INVALID_TILE_SIZE", "瓦片尺寸必须是 16、24、32 或 48")
	}
	if m.Width < 1 || m.Width > 1024 || m.Height < 1 || m.Height > 1024 {
		return apperr.New("INVALID_MAP", "地图尺寸必须在 1 到 1024 格之间")
	}

	if m.Tilesets == nil {
		m.Tilesets = []MapTileset{}
	}
	if len(m.Tilesets) > 32 {
		return apperr.New("INVALID_MAP", "一张地图最多使用 32 个瓦片集")
	}
	seenSets := map[string]bool{}
	for i := range m.Tilesets {
		set := &m.Tilesets[i]
		if set.ID == "" {
			set.ID = NewID()
		}
		if !ValidID(set.ID) || seenSets[set.ID] {
			return apperr.New("INVALID_MAP", "瓦片集 ID 不合法或重复")
		}
		seenSets[set.ID] = true
		if !ValidID(set.AssetID) {
			return apperr.New("INVALID_MAP", "瓦片集引用的素材 ID 不合法")
		}
		if set.TileSize == 0 {
			set.TileSize = m.TileSize
		}
		if !isTileSize(set.TileSize) {
			return apperr.New("INVALID_MAP", "瓦片集的瓦片尺寸不合法")
		}
		if set.Columns < 1 || set.TileCount < 1 || set.TileCount > 4096 {
			return apperr.New("INVALID_MAP", "瓦片集的行列数不合法")
		}
	}

	if m.Layers == nil || len(m.Layers) == 0 {
		return apperr.New("INVALID_MAP", "地图至少需要一个图层")
	}
	if len(m.Layers) > 32 {
		return apperr.New("INVALID_MAP", "地图最多 32 个图层")
	}
	seenLayers := map[string]bool{}
	for i := range m.Layers {
		layer := &m.Layers[i]
		if layer.ID == "" {
			layer.ID = NewID()
		}
		if !ValidID(layer.ID) || seenLayers[layer.ID] {
			return apperr.New("INVALID_MAP", "图层 ID 不合法或重复")
		}
		seenLayers[layer.ID] = true
		layer.Name = strings.TrimSpace(layer.Name)
		if layer.Name == "" {
			layer.Name = layer.Type
		}
		kind, err := choice(layer.Type, mapLayerTypes, MapGround)
		if err != nil {
			return apperr.New("INVALID_MAP", "图层类型不合法").WithDetail(layer.Type)
		}
		layer.Type = kind
		if layer.Elevation < 0 || layer.Elevation > 16 {
			layer.Elevation = 0
		}
		if layer.Opacity <= 0 || layer.Opacity > 1 {
			layer.Opacity = 1
		}
		if layer.Autotile != nil {
			if layer.Autotile.Mode != "16" && layer.Autotile.Mode != "47" {
				layer.Autotile.Mode = "16"
			}
			if layer.Autotile.Set < 0 {
				layer.Autotile.Set = 0
			}
			if layer.Autotile.Base < 0 {
				layer.Autotile.Base = 0
			}
		}
		layer.Tiles = normalizeMapTiles(layer.Tiles, m, layer.Type)
	}

	if m.Objects == nil {
		m.Objects = []MapObject{}
	}
	if len(m.Objects) > 4096 {
		return apperr.New("INVALID_MAP", "地图对象过多")
	}
	seenObjects := map[string]bool{}
	for i := range m.Objects {
		obj := &m.Objects[i]
		if obj.ID == "" {
			obj.ID = NewID()
		}
		if !ValidID(obj.ID) || seenObjects[obj.ID] {
			return apperr.New("INVALID_MAP", "地图对象 ID 不合法或重复")
		}
		seenObjects[obj.ID] = true
		obj.Name = strings.TrimSpace(obj.Name)
		if obj.Name == "" {
			obj.Name = obj.Kind
		}
		kind, err := choice(obj.Kind, mapObjectKinds, MapObjectProp)
		if err != nil {
			return apperr.New("INVALID_MAP", "地图对象类型不合法").WithDetail(obj.Kind)
		}
		obj.Kind = kind
		obj.X = clampFloat(obj.X, 0, float64(m.Width))
		obj.Y = clampFloat(obj.Y, 0, float64(m.Height))
		obj.W = clampFloat(obj.W, 0.25, float64(m.Width))
		obj.H = clampFloat(obj.H, 0.25, float64(m.Height))
		if obj.Elevation < 0 || obj.Elevation > 16 {
			obj.Elevation = 0
		}
		if obj.AssetID != "" && !ValidID(obj.AssetID) {
			obj.AssetID = ""
		}
		if obj.Color != "" && !colorPattern.MatchString(obj.Color) {
			obj.Color = ""
		}
		if obj.Properties == nil {
			obj.Properties = map[string]string{}
		}
	}

	if m.Settings.HeightStep <= 0 || m.Settings.HeightStep > 128 {
		m.Settings.HeightStep = 16
	}
	if !colorPattern.MatchString(m.Settings.Ambient) {
		m.Settings.Ambient = "#8090a0"
	}
	m.SchemaVersion = SchemaVersion
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

// normalizeMapTiles drops out-of-bounds or dangling cells, deduplicates by
// coordinate (last write wins) and returns them sorted for stable diffs.
func normalizeMapTiles(tiles []MapTile, m *GameMap, layerType string) []MapTile {
	if tiles == nil {
		return []MapTile{}
	}
	type point struct{ x, y int }
	seen := map[point]int{}
	out := make([]MapTile, 0, len(tiles))
	for _, tile := range tiles {
		if tile.X < 0 || tile.Y < 0 || tile.X >= m.Width || tile.Y >= m.Height {
			continue
		}
		if layerType != MapCollision {
			if tile.Set < 0 || tile.Set >= len(m.Tilesets) {
				continue
			}
			if tile.Tile < 0 || tile.Tile >= m.Tilesets[tile.Set].TileCount {
				continue
			}
		}
		switch tile.Rotate {
		case 90, 180, 270:
		default:
			tile.Rotate = 0
		}
		if layerType == MapCollision {
			// Collision cells only need their occupancy; normalise the rest.
			tile.Set = 0
			tile.Tile = 0
			tile.FlipH = false
			tile.FlipV = false
			tile.Rotate = 0
		}
		key := point{tile.X, tile.Y}
		if index, ok := seen[key]; ok {
			out[index] = tile
			continue
		}
		seen[key] = len(out)
		out = append(out, tile)
	}
	slices.SortStableFunc(out, func(a, b MapTile) int {
		if a.Y != b.Y {
			return a.Y - b.Y
		}
		return a.X - b.X
	})
	return out
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ValidatePalette normalises defaults and validates a palette.
func ValidatePalette(p *Palette) error {
	if p == nil {
		return apperr.New("INVALID_PALETTE", "调色板数据为空")
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return apperr.New("INVALID_PALETTE", "调色板名称不能为空")
	}
	if p.ID == "" {
		p.ID = NewID()
	}
	if !ValidID(p.ID) {
		return apperr.New("INVALID_PALETTE", "调色板 ID 不合法")
	}
	if len(p.Colors) < 2 || len(p.Colors) > 64 {
		return apperr.New("INVALID_PALETTE", "调色板需要 2 到 64 种颜色")
	}
	for i := range p.Colors {
		p.Colors[i] = strings.ToLower(strings.TrimSpace(p.Colors[i]))
		if !colorPattern.MatchString(p.Colors[i]) {
			return apperr.Newf("INVALID_PALETTE", "颜色格式不合法：%s", p.Colors[i])
		}
	}
	p.SchemaVersion = SchemaVersion
	return nil
}
