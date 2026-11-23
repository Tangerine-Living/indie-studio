package assets

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pixelbench/internal/apperr"
	"pixelbench/internal/imageproc"
	"pixelbench/internal/project"
)

// PostProcessMeta is stored beside a pixel-perfect output version so the same
// palette, dither and seam result can be audited or reused.
type PostProcessMeta struct {
	Options imageproc.Options    `json:"options"`
	Palette []string             `json:"palette"`
	Seam    imageproc.SeamReport `json:"seam"`
}

// LoadAssetImage returns the picture the post-processor should clean: the
// current version when one exists, otherwise the first animation frame, the
// generated composite, or the first original import.
func LoadAssetImage(store *project.Store, assetID string) (image.Image, string, error) {
	asset, err := store.GetAsset(assetID)
	if err != nil {
		return nil, "", err
	}
	path, err := assetImagePath(store.Root(), asset)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", apperr.Wrap("ASSET_SOURCE_MISSING", "无法打开素材图片", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, "", apperr.Wrap("INVALID_IMAGE", "无法解析素材图片", err)
	}
	return img, path, nil
}

func assetImagePath(root string, asset project.Asset) (string, error) {
	dir := project.AssetDir(root, asset.ID)
	// Animated assets are processed per frame, never through their sheet.
	if asset.Animation != nil {
		first := filepath.Join(dir, "frames", "000.png")
		if fileExists(first) {
			return first, nil
		}
		frames, _ := filepath.Glob(filepath.Join(dir, "frames", "*.png"))
		if len(frames) > 0 {
			sort.Strings(frames)
			return frames[0], nil
		}
	}
	if asset.CurrentVersion != "" {
		path := filepath.Join(dir, "versions", asset.CurrentVersion, "output.png")
		if fileExists(path) {
			return path, nil
		}
	}
	generated := filepath.Join(dir, "source", "generated.png")
	if fileExists(generated) {
		return generated, nil
	}
	sourceDir := filepath.Join(dir, "source")
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return "", apperr.New("ASSET_SOURCE_MISSING", "素材没有可处理的图片")
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := supportedExt[strings.ToLower(filepath.Ext(name))]; !ok {
			continue
		}
		if strings.HasSuffix(name, ".base.png") || strings.HasSuffix(name, ".roof.png") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "", apperr.New("ASSET_SOURCE_MISSING", "素材没有可处理的图片")
	}
	sort.Strings(names)
	return filepath.Join(sourceDir, names[0]), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// ProcessAsset loads the asset image and runs the pixel-perfect chain.
func ProcessAsset(store *project.Store, assetID string, opts imageproc.Options) (imageproc.Result, error) {
	src, _, err := LoadAssetImage(store, assetID)
	if err != nil {
		return imageproc.Result{}, err
	}
	return imageproc.Process(src, opts), nil
}

// SaveProcessedAsset runs the chain and stores the result as the next
// immutable version, recording the options, palette and seam report.
func SaveProcessedAsset(
	store *project.Store,
	assetID string,
	opts imageproc.Options,
) (project.Asset, imageproc.Result, error) {
	src, _, err := LoadAssetImage(store, assetID)
	if err != nil {
		return project.Asset{}, imageproc.Result{}, err
	}
	result := imageproc.Process(src, opts)
	var buf bytes.Buffer
	if err := png.Encode(&buf, result.Image); err != nil {
		return project.Asset{}, imageproc.Result{}, apperr.Wrap("IO", "无法编码处理结果", err)
	}
	srcW, srcH := src.Bounds().Dx(), src.Bounds().Dy()
	saved, err := saveVersion(store, assetID, buf.Bytes(), "postprocess", func(meta *VersionMeta) {
		meta.Process = &PostProcessMeta{
			Options: result.Options,
			Palette: result.Colors,
			Seam:    result.Seam,
		}
	}, func(asset *project.Asset) {
		// A downscaled AI render would otherwise keep a pivot from the source
		// pixel grid, and the adopted sprite would anchor far off-canvas.
		asset.Pivot = scalePivot(asset.Pivot, srcW, srcH, result)
	})
	if err != nil {
		return project.Asset{}, imageproc.Result{}, err
	}
	return saved, result, nil
}

// scalePivot maps a source-image pivot onto the processed result, accounting
// for both the resize and the outline padding added by the pipeline.
func scalePivot(pivot project.Point, srcW, srcH int, result imageproc.Result) project.Point {
	if srcW <= 0 || srcH <= 0 || result.Width <= 0 || result.Height <= 0 {
		return pivot
	}
	pad := result.Pad
	innerW := result.Width - 2*pad
	innerH := result.Height - 2*pad
	clamp := func(v, max int) int {
		if v < 0 {
			return 0
		}
		if v > max {
			return max
		}
		return v
	}
	return project.Point{
		X: clamp(int(math.Round(float64(pivot.X)*float64(innerW)/float64(srcW)))+pad, result.Width-1),
		Y: clamp(int(math.Round(float64(pivot.Y)*float64(innerH)/float64(srcH)))+pad, result.Height-1),
	}
}
