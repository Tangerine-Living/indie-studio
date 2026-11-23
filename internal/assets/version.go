package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// MaxVersionBytes caps a saved editor result.
const MaxVersionBytes = 32 << 20

// VersionMeta is the sidecar written next to every immutable version.
type VersionMeta struct {
	Kind      string    `json:"kind"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"createdAt"`
	App       string    `json:"app"`
	// Sheet is set for sprite-sheet versions only.
	Sheet *AnimationInfo `json:"sheet,omitempty"`
	// Generation is set for procedurally / AI generated versions.
	Generation *GenerationMeta `json:"generation,omitempty"`
	// Process records the pixel-perfect post-processing chain, when applied.
	Process *PostProcessMeta `json:"process,omitempty"`
}

// SaveVersion stores a PNG as the asset's next immutable version, refreshes the
// thumbnail and updates the manifest's current-version pointer.
func SaveVersion(store *project.Store, assetID string, pngData []byte) (project.Asset, error) {
	return saveVersion(store, assetID, pngData, "edit", nil, nil)
}

// saveVersion is the shared immutable-version writer. decorate may add
// kind-specific metadata before the sidecar is written; adjust may remap
// manifest fields (such as the pivot) to the new image before saving.
func saveVersion(
	store *project.Store,
	assetID string,
	pngData []byte,
	kind string,
	decorate func(*VersionMeta),
	adjust func(*project.Asset),
) (project.Asset, error) {
	if len(pngData) == 0 {
		return project.Asset{}, apperr.New("INVALID_VERSION", "没有可保存的图片")
	}
	if len(pngData) > MaxVersionBytes {
		return project.Asset{}, apperr.New("INVALID_VERSION", "图片超过 32 MB，无法保存")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(pngData))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return project.Asset{}, apperr.New("INVALID_VERSION", "保存的内容不是有效的 PNG")
	}

	asset, err := store.GetAsset(assetID)
	if err != nil {
		return project.Asset{}, err
	}

	next := nextVersionID(asset)
	dir := filepath.Join(project.AssetDir(store.Root(), asset.ID), "versions", next)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return project.Asset{}, apperr.Wrap("IO", "无法创建版本目录", err)
	}
	rollback := func() { _ = os.RemoveAll(dir) }

	if err := fsutil.WriteFileAtomic(filepath.Join(dir, "output.png"), pngData, 0o644); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法写入版本图片", err)
	}
	if err := writeThumbFromBytes(pngData, filepath.Join(project.AssetDir(store.Root(), asset.ID), "thumb.png")); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法更新缩略图", err)
	}

	sum := sha256.Sum256(pngData)
	hash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	meta := VersionMeta{
		Kind:      kind,
		Width:     cfg.Width,
		Height:    cfg.Height,
		Hash:      hash,
		CreatedAt: now,
		App:       "pixelbench",
	}
	if decorate != nil {
		decorate(&meta)
	}
	if err := fsutil.WriteJSONAtomic(filepath.Join(dir, "meta.json"), meta, 0o644); err != nil {
		rollback()
		return project.Asset{}, apperr.Wrap("IO", "无法写入版本元数据", err)
	}

	// A single-image version replaces whatever animation the asset had; the
	// sheet stays in history but the current version is one still frame.
	oldDim := asset.Dimensions
	asset.Animation = nil
	asset.Dimensions = project.Dimensions{W: cfg.Width, H: cfg.Height}
	asset.Versions = append(asset.Versions, project.AssetVersion{
		ID:        next,
		CreatedAt: now,
		Hash:      hash,
		MetaFile:  "versions/" + next + "/meta.json",
		Size:      int64(len(pngData)),
	})
	asset.CurrentVersion = next
	if adjust != nil {
		adjust(&asset)
	} else if oldDim.W > 0 && oldDim.H > 0 && (oldDim.W != cfg.Width || oldDim.H != cfg.Height) {
		// Plain editor saves keep the anchor in the same relative spot; the
		// post-process path has its own pivot remap that accounts for padding.
		px := int(math.Round(float64(asset.Pivot.X) * float64(cfg.Width) / float64(oldDim.W)))
		py := int(math.Round(float64(asset.Pivot.Y) * float64(cfg.Height) / float64(oldDim.H)))
		asset.Pivot = project.Point{
			X: max(0, min(cfg.Width-1, px)),
			Y: max(0, min(cfg.Height-1, py)),
		}
	}

	saved, err := store.SaveAsset(asset)
	if err != nil {
		rollback()
		return project.Asset{}, err
	}
	return saved, nil
}

func nextVersionID(asset project.Asset) string {
	max := 0
	for _, version := range asset.Versions {
		if n, err := strconv.Atoi(version.ID); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%04d", max+1)
}
