package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// AnimationInfo describes how a sprite-sheet PNG is laid out. It is both the
// input of SaveSheet and the payload written into the version meta.
type AnimationInfo struct {
	FrameWidth  int  `json:"frameWidth"`
	FrameHeight int  `json:"frameHeight"`
	FrameCount  int  `json:"frameCount"`
	Columns     int  `json:"columns"`
	Rows        int  `json:"rows"`
	Margin      int  `json:"margin"`
	Spacing     int  `json:"spacing"`
	FPS         int  `json:"fps"`
	Loop        bool `json:"loop"`
	// Durations holds per-frame hold times in milliseconds. Empty means every
	// frame uses the uniform duration derived from FPS.
	Durations []int `json:"durations,omitempty"`
	// Tags names the animation clips this sheet belongs to (e.g. idle, walk).
	Tags []string `json:"tags,omitempty"`
}

const (
	maxSheetCell       = 4096
	minFrameDurationMs = 10
	maxFrameDurationMs = 10000
	maxAnimationTags   = 12
)

func (info AnimationInfo) normalized() (AnimationInfo, error) {
	invalid := func(detail string) (AnimationInfo, error) {
		return AnimationInfo{}, apperr.New("INVALID_SHEET", "精灵表参数不正确").WithDetail(detail)
	}
	if info.FrameWidth < 1 || info.FrameWidth > maxSheetCell ||
		info.FrameHeight < 1 || info.FrameHeight > maxSheetCell {
		return invalid("帧尺寸必须在 1–4096 像素之间")
	}
	if info.FrameCount < 1 || info.FrameCount > maxSheetCell {
		return invalid("帧数必须在 1–4096 之间")
	}
	if info.Columns < 1 || info.Columns > info.FrameCount {
		return invalid("列数必须在 1 和帧数之间")
	}
	if info.Margin < 0 || info.Margin > maxSheetCell || info.Spacing < 0 || info.Spacing > maxSheetCell {
		return invalid("外边距与间隙不能为负")
	}
	if info.FPS < 1 {
		info.FPS = 8
	}
	if info.FPS > 60 {
		info.FPS = 60
	}
	info.Rows = (info.FrameCount + info.Columns - 1) / info.Columns
	info.Durations = normalizeDurations(info.Durations, info.FrameCount, frameDurationMs(info))
	info.Tags = normalizeTags(info.Tags)
	return info, nil
}

// normalizeDurations pads or truncates raw hold times to one per frame, with
// the uniform FPS duration filling missing or invalid entries.
func normalizeDurations(raw []int, count, fallback int) []int {
	if len(raw) == 0 {
		return nil
	}
	out := make([]int, count)
	for index := range out {
		duration := fallback
		if index < len(raw) && raw[index] > 0 {
			duration = raw[index]
			if duration < minFrameDurationMs {
				duration = minFrameDurationMs
			}
			if duration > maxFrameDurationMs {
				duration = maxFrameDurationMs
			}
		}
		out[index] = duration
	}
	return out
}

// normalizeTags trims, de-duplicates and caps clip names.
func normalizeTags(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, tag := range raw {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if runes := []rune(tag); len(runes) > 24 {
			tag = string(runes[:24])
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
		if len(out) == maxAnimationTags {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SheetWidth is the exact pixel width a sheet with this layout must have.
func (info AnimationInfo) SheetWidth() int {
	return info.Margin*2 + info.Columns*info.FrameWidth + max(0, info.Columns-1)*info.Spacing
}

// SheetHeight is the exact pixel height a sheet with this layout must have.
func (info AnimationInfo) SheetHeight() int {
	return info.Margin*2 + info.Rows*info.FrameHeight + max(0, info.Rows-1)*info.Spacing
}

// FrameOrigin returns the top-left pixel of a cell, row-major.
func (info AnimationInfo) FrameOrigin(index int) (x, y int) {
	column := index % info.Columns
	row := index / info.Columns
	return info.Margin + column*(info.FrameWidth+info.Spacing),
		info.Margin + row*(info.FrameHeight+info.Spacing)
}

// SaveSheet stores a sprite sheet as the asset's next immutable version and
// derives per-frame PNGs, Aseprite/TexturePacker JSON and the thumbnail from
// it. Frames live in assets/<id>/frames/ so the editor can reopen frame 0.
func SaveSheet(store *project.Store, assetID string, pngData []byte, raw AnimationInfo) (project.Asset, error) {
	if len(pngData) == 0 {
		return project.Asset{}, apperr.New("INVALID_SHEET", "没有可保存的精灵表")
	}
	if len(pngData) > MaxVersionBytes {
		return project.Asset{}, apperr.New("INVALID_SHEET", "精灵表超过 32 MB，无法保存")
	}
	info, err := raw.normalized()
	if err != nil {
		return project.Asset{}, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(pngData))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return project.Asset{}, apperr.New("INVALID_SHEET", "保存的内容不是有效的 PNG")
	}
	if cfg.Width != info.SheetWidth() || cfg.Height != info.SheetHeight() {
		return project.Asset{}, apperr.New("INVALID_SHEET", "精灵表尺寸与布局不一致").
			WithDetail(fmt.Sprintf("图片为 %d×%d，按布局应为 %d×%d",
				cfg.Width, cfg.Height, info.SheetWidth(), info.SheetHeight()))
	}
	src, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return project.Asset{}, apperr.Wrap("INVALID_SHEET", "无法读取精灵表图片", err)
	}

	asset, err := store.GetAsset(assetID)
	if err != nil {
		return project.Asset{}, err
	}
	assetDir := project.AssetDir(store.Root(), asset.ID)
	next := nextVersionID(asset)
	dir := filepath.Join(assetDir, "versions", next)
	framesDir := filepath.Join(assetDir, "frames")
	framesTmp := filepath.Join(assetDir, "frames.new")

	rollback := func() {
		_ = os.RemoveAll(dir)
		_ = os.RemoveAll(framesTmp)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return project.Asset{}, apperr.Wrap("IO", "无法创建版本目录", err)
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "output.png"), pngData, 0o644); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法写入版本图片", err)
	}

	// Per-frame PNGs are derived data; the sheet version stays the source.
	_ = os.RemoveAll(framesTmp)
	if err := os.MkdirAll(framesTmp, 0o755); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法创建帧目录", err)
	}
	var first image.Image
	for index := 0; index < info.FrameCount; index++ {
		x, y := info.FrameOrigin(index)
		cell := image.NewNRGBA(image.Rect(0, 0, info.FrameWidth, info.FrameHeight))
		draw.Draw(cell, cell.Bounds(), src, image.Pt(x, y), draw.Src)
		if index == 0 {
			first = cell
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, cell); err != nil {
			rollback()
			return project.Asset{}, apperr.Wrap("IO", "无法编码帧图片", err)
		}
		name := fmt.Sprintf("%03d.png", index)
		if err := fsutil.WriteFileAtomic(filepath.Join(framesTmp, name), buf.Bytes(), 0o644); err != nil {
			rollback()
			return project.Asset{}, apperr.Wrap("IO", "无法写入帧图片", err)
		}
	}

	_ = os.RemoveAll(framesDir)
	if err := os.Rename(framesTmp, framesDir); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法更新帧目录", err)
	}

	if err := writeThumbFromImage(first, filepath.Join(assetDir, "thumb.png")); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法更新缩略图", err)
	}

	sum := sha256.Sum256(pngData)
	hash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	meta := VersionMeta{
		Kind:      "sheet",
		Width:     cfg.Width,
		Height:    cfg.Height,
		Hash:      hash,
		CreatedAt: now,
		App:       "pixelbench",
		Sheet:     &info,
	}
	if err := fsutil.WriteJSONAtomic(filepath.Join(dir, "meta.json"), meta, 0o644); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法写入版本元数据", err)
	}
	if err := fsutil.WriteJSONAtomic(filepath.Join(dir, "frames-aseprite.json"), asepriteJSON(info), 0o644); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法写入 Aseprite JSON", err)
	}
	if err := fsutil.WriteJSONAtomic(filepath.Join(dir, "frames-texturepacker.json"), texturePackerJSON(info), 0o644); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法写入 TexturePacker JSON", err)
	}

	tags := info.Tags
	if len(tags) == 0 && asset.Animation != nil {
		tags = asset.Animation.Tags
	}
	if tags == nil {
		tags = []string{}
	}
	asset.Animation = &project.Animation{
		FPS:            info.FPS,
		Loop:           info.Loop,
		Tags:           tags,
		FrameWidth:     info.FrameWidth,
		FrameHeight:    info.FrameHeight,
		FrameCount:     info.FrameCount,
		Columns:        info.Columns,
		FrameDurations: info.Durations,
	}
	// A sheet usually changes the asset from a full atlas to one frame; keep
	// the anchor in the same relative spot instead of leaving it off-canvas.
	oldDim := asset.Dimensions
	if oldDim.W > 0 && oldDim.H > 0 && (oldDim.W != info.FrameWidth || oldDim.H != info.FrameHeight) {
		px := int(math.Round(float64(asset.Pivot.X) * float64(info.FrameWidth) / float64(oldDim.W)))
		py := int(math.Round(float64(asset.Pivot.Y) * float64(info.FrameHeight) / float64(oldDim.H)))
		asset.Pivot = project.Point{
			X: max(0, min(info.FrameWidth-1, px)),
			Y: max(0, min(info.FrameHeight-1, py)),
		}
	}
	asset.Dimensions = project.Dimensions{W: info.FrameWidth, H: info.FrameHeight}
	asset.Versions = append(asset.Versions, project.AssetVersion{
		ID:        next,
		CreatedAt: now,
		Hash:      hash,
		MetaFile:  "versions/" + next + "/meta.json",
		Size:      int64(len(pngData)),
	})
	asset.CurrentVersion = next

	saved, err := store.SaveAsset(asset)
	if err != nil {
		rollback()
		return project.Asset{}, err
	}
	return saved, nil
}

func frameDurationMs(info AnimationInfo) int {
	fps := info.FPS
	if fps < 1 {
		fps = 8
	}
	return int(math.Round(1000.0 / float64(fps)))
}

// frameDurations returns one hold time per frame, falling back to the uniform
// FPS duration for frames without an explicit value.
func frameDurations(info AnimationInfo) []int {
	out := make([]int, info.FrameCount)
	uniform := frameDurationMs(info)
	for index := range out {
		duration := uniform
		if index < len(info.Durations) && info.Durations[index] > 0 {
			duration = info.Durations[index]
		}
		out[index] = duration
	}
	return out
}

// animationTagNames returns the clip names to write into the metadata, with a
// single "default" tag when the asset has no named clips.
func animationTagNames(info AnimationInfo) []string {
	if len(info.Tags) > 0 {
		return info.Tags
	}
	return []string{"default"}
}

func asepriteJSON(info AnimationInfo) map[string]any {
	durations := frameDurations(info)
	size := map[string]int{"w": info.SheetWidth(), "h": info.SheetHeight()}
	frames := map[string]any{}
	for index := 0; index < info.FrameCount; index++ {
		x, y := info.FrameOrigin(index)
		frames[fmt.Sprintf("%03d", index)] = map[string]any{
			"frame":            map[string]int{"x": x, "y": y, "w": info.FrameWidth, "h": info.FrameHeight},
			"rotated":          false,
			"trimmed":          false,
			"spriteSourceSize": map[string]int{"x": 0, "y": 0, "w": info.FrameWidth, "h": info.FrameHeight},
			"sourceSize":       map[string]int{"w": info.FrameWidth, "h": info.FrameHeight},
			"duration":         durations[index],
		}
	}
	tags := make([]any, 0, len(animationTagNames(info)))
	for _, name := range animationTagNames(info) {
		tags = append(tags, map[string]any{
			"name":      name,
			"from":      0,
			"to":        info.FrameCount - 1,
			"direction": "forward",
		})
	}
	return map[string]any{
		"frames": frames,
		"meta": map[string]any{
			"app":       "pixelbench",
			"version":   "1.0",
			"image":     "output.png",
			"format":    "RGBA8888",
			"size":      size,
			"scale":     "1",
			"frameTags": tags,
			"layers":    []any{map[string]any{"name": "Layer 1", "opacity": 255, "blendMode": "normal"}},
			"slices":    []any{},
		},
	}
}

func texturePackerJSON(info AnimationInfo) map[string]any {
	frames := map[string]any{}
	animation := make([]string, 0, info.FrameCount)
	for index := 0; index < info.FrameCount; index++ {
		x, y := info.FrameOrigin(index)
		name := fmt.Sprintf("%03d", index)
		animation = append(animation, name)
		frames[name] = map[string]any{
			"frame":            map[string]int{"x": x, "y": y, "w": info.FrameWidth, "h": info.FrameHeight},
			"rotated":          false,
			"trimmed":          false,
			"spriteSourceSize": map[string]int{"x": 0, "y": 0, "w": info.FrameWidth, "h": info.FrameHeight},
			"sourceSize":       map[string]int{"w": info.FrameWidth, "h": info.FrameHeight},
		}
	}
	animations := map[string]any{}
	for _, name := range animationTagNames(info) {
		animations[name] = animation
	}
	return map[string]any{
		"frames":     frames,
		"animations": animations,
		"meta": map[string]any{
			"app":     "pixelbench",
			"version": "1.0",
			"image":   "output.png",
			"format":  "RGBA8888",
			"size":    map[string]int{"w": info.SheetWidth(), "h": info.SheetHeight()},
			"scale":   "1",
		},
	}
}
