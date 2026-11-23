// Package assets imports image files into a project: it keeps the original,
// hashes it for duplicate detection, renders a thumbnail and writes the
// manifest. Heavy pixel processing will grow here in later milestones.
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// MaxSourceBytes guards against accidentally importing huge files.
const MaxSourceBytes = 128 << 20

const thumbMax = 256

var supportedExt = map[string]string{
	".png":  "png",
	".jpg":  "jpeg",
	".jpeg": "jpeg",
	".gif":  "gif",
	".webp": "webp",
	".bmp":  "bmp",
}

var allowedTypes = map[string]bool{
	project.AssetSprite:   true,
	project.AssetBuilding: true,
	project.AssetTile:     true,
	project.AssetProp:     true,
	project.AssetUI:       true,
	project.AssetFX:       true,
	project.AssetOther:    true,
}

// Duplicate describes a file that is already in the library.
type Duplicate struct {
	Path    string `json:"path"`
	AssetID string `json:"assetId"`
	Name    string `json:"name"`
}

// Failure describes one file that could not be imported.
type Failure struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Summary is the per-file outcome of a batch import.
type Summary struct {
	Imported   []project.Asset `json:"imported"`
	Duplicates []Duplicate     `json:"duplicates"`
	Failed     []Failure       `json:"failed"`
}

// Import copies each supported image into the project, deduplicating by
// content hash. One bad file does not abort the batch.
func Import(store *project.Store, paths []string, assetType string) (Summary, error) {
	summary := Summary{
		Imported:   []project.Asset{},
		Duplicates: []Duplicate{},
		Failed:     []Failure{},
	}
	if len(paths) == 0 {
		return summary, apperr.New("INVALID_IMPORT", "请选择要导入的图片")
	}
	if assetType == "" {
		assetType = project.AssetOther
	}
	if !allowedTypes[assetType] {
		return summary, apperr.New("INVALID_ASSET", "素材类型不合法").WithDetail(assetType)
	}

	root := store.Root()
	session := store.Session()
	paletteID := session.Project.Settings.DefaultPaletteID

	known := map[string]project.Asset{}
	if existing, err := store.ListAssets(); err == nil {
		for _, asset := range existing {
			hash := asset.SourceHash
			if hash == "" {
				hash, _ = hashSourceDir(project.AssetDir(root, asset.ID))
			}
			if hash != "" {
				known[hash] = asset
			}
		}
	}

	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		asset, duplicate, failure := importOne(store, root, path, assetType, paletteID, known)
		switch {
		case duplicate != nil:
			summary.Duplicates = append(summary.Duplicates, *duplicate)
		case failure != nil:
			summary.Failed = append(summary.Failed, *failure)
		case asset != nil:
			summary.Imported = append(summary.Imported, *asset)
			known[asset.SourceHash] = *asset
		}
	}
	return summary, nil
}

func importOne(
	store *project.Store,
	root, path, assetType, paletteID string,
	known map[string]project.Asset,
) (*project.Asset, *Duplicate, *Failure) {
	fail := func(message string) (*project.Asset, *Duplicate, *Failure) {
		return nil, nil, &Failure{Path: path, Message: message}
	}

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fail("文件不存在或不是普通文件")
	}
	if info.Size() > MaxSourceBytes {
		return fail("文件超过 128 MB，暂不支持导入")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := supportedExt[ext]; !ok {
		return fail("不支持的格式（支持 PNG / JPEG / GIF / WebP / BMP）")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fail("无法读取文件：" + err.Error())
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if existing, ok := known[hash]; ok {
		return nil, &Duplicate{Path: path, AssetID: existing.ID, Name: existing.Name}, nil
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return fail("无法解析图片，文件可能已损坏")
	}

	id := project.NewID()
	dir := project.AssetDir(root, id)
	sourceDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return fail("无法创建素材目录：" + err.Error())
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	filename := filepath.Base(path)
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		cleanup()
		return fail("文件名不合法")
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(sourceDir, filename), data, 0o644); err != nil {
		cleanup()
		return fail("无法保存原图：" + err.Error())
	}

	if err := writeThumbFromBytes(data, filepath.Join(dir, "thumb.png")); err != nil {
		cleanup()
		return fail("无法生成缩略图：" + err.Error())
	}

	now := time.Now().UTC()
	asset := project.Asset{
		SchemaVersion: project.SchemaVersion,
		ID:            id,
		Name:          assetName(filename),
		Type:          assetType,
		Tags:          []string{},
		Status:        project.AssetDraft,
		Dimensions:    project.Dimensions{W: cfg.Width, H: cfg.Height},
		Pivot:         project.Point{X: cfg.Width / 2, Y: cfg.Height - 1},
		PaletteID:     paletteID,
		QuestIDs:      []string{},
		License:       "imported",
		Source:        project.AssetSource{Kind: "imported"},
		SourceHash:    hash,
		Versions:      []project.AssetVersion{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	saved, err := store.SaveAsset(asset)
	if err != nil {
		cleanup()
		return fail("无法写入素材清单：" + err.Error())
	}
	return &saved, nil, nil
}

func assetName(filename string) string {
	name := strings.TrimSpace(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if name == "" {
		return "未命名素材"
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return name
}

func writeThumbFromBytes(data []byte, dst string) error {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return writeThumbFromImage(src, dst)
}

func writeThumbFromImage(src image.Image, dst string) error {
	thumb := fitThumb(src, thumbMax)
	var buf bytes.Buffer
	if err := png.Encode(&buf, thumb); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(dst, buf.Bytes(), 0o644)
}

// fitThumb keeps small pixel art crisp (integer nearest upscaling, at most 4x)
// and area-averages larger images down to the thumbnail box.
func fitThumb(src image.Image, max int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	if w <= max && h <= max {
		scale := max / w
		if other := max / h; other < scale {
			scale = other
		}
		if scale < 1 {
			scale = 1
		}
		if scale > 4 {
			scale = 4
		}
		dst := image.NewNRGBA(image.Rect(0, 0, w*scale, h*scale))
		for y := 0; y < dst.Bounds().Dy(); y++ {
			for x := 0; x < dst.Bounds().Dx(); x++ {
				dst.Set(x, y, src.At(b.Min.X+x/scale, b.Min.Y+y/scale))
			}
		}
		return dst
	}

	tw, th := max, max
	if w >= h {
		tw = max
		th = h * max / w
	} else {
		th = max
		tw = w * max / h
	}
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, tw, th))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

// hashSourceDir hashes the original files of an asset that predates the
// SourceHash field, so older projects still deduplicate.
func hashSourceDir(dir string) (string, error) {
	sourceDir := filepath.Join(dir, "source")
	var files []string
	err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		h.Write(data)
	}
	if len(files) == 0 {
		return "", nil
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
