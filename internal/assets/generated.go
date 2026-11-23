package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// GeneratedInput is a rendered generated asset ready to be written into the
// project: a composite plus optional base/roof layers and reproducibility data.
type GeneratedInput struct {
	Name      string
	Type      string
	PaletteID string
	Width     int
	Height    int
	Pivot     project.Point
	PNG       []byte
	BasePNG   []byte
	RoofPNG   []byte
	Generator string
	// Provider overrides the source provider (default "procedural").
	Provider  string
	Seed      int64
	Options   any
	Collision AnimationRect
}

// AnimationRect is a pixel rectangle stored in generation metadata.
type AnimationRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// GenerationMeta pins how a version was generated.
type GenerationMeta struct {
	Generator string          `json:"generator"`
	Seed      int64           `json:"seed"`
	Options   json.RawMessage `json:"options,omitempty"`
	Pivot     project.Point   `json:"pivot"`
	Collision AnimationRect   `json:"collision"`
}

func validateGeneratedPNG(data []byte) (int, int, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, apperr.New("INVALID_GENERATED", "生成的图片无效")
	}
	return cfg.Width, cfg.Height, nil
}

// CreateGeneratedAsset writes a generated result as a new draft asset with one
// immutable version, keeping base/roof layers next to the composite.
func CreateGeneratedAsset(store *project.Store, input GeneratedInput) (project.Asset, error) {
	if len(input.PNG) == 0 {
		return project.Asset{}, apperr.New("INVALID_GENERATED", "没有可保存的生成结果")
	}
	if len(input.PNG) > MaxVersionBytes {
		return project.Asset{}, apperr.New("INVALID_GENERATED", "生成结果超过 32 MB")
	}
	width, height, err := validateGeneratedPNG(input.PNG)
	if err != nil {
		return project.Asset{}, err
	}
	if input.Width > 0 && input.Height > 0 && (width != input.Width || height != input.Height) {
		return project.Asset{}, apperr.New("INVALID_GENERATED", "生成结果的尺寸与参数不一致").
			WithDetail(fmt.Sprintf("图片 %d×%d，参数 %d×%d", width, height, input.Width, input.Height))
	}
	if len(input.BasePNG) > 0 {
		if bw, bh, err := validateGeneratedPNG(input.BasePNG); err != nil {
			return project.Asset{}, err
		} else if bw != width || bh != height {
			return project.Asset{}, apperr.New("INVALID_GENERATED", "分层图片尺寸不一致")
		}
	}
	if len(input.RoofPNG) > 0 {
		if rw, rh, err := validateGeneratedPNG(input.RoofPNG); err != nil {
			return project.Asset{}, err
		} else if rw != width || rh != height {
			return project.Asset{}, apperr.New("INVALID_GENERATED", "分层图片尺寸不一致")
		}
	}

	options, marshalErr := json.Marshal(input.Options)
	if marshalErr != nil {
		return project.Asset{}, apperr.Wrap("INVALID_GENERATED", "无法记录生成参数", marshalErr)
	}

	id := project.NewID()
	dir := project.AssetDir(store.Root(), id)
	sourceDir := filepath.Join(dir, "source")
	versionDir := filepath.Join(dir, "versions", "0001")
	for _, path := range []string{sourceDir, versionDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return project.Asset{}, apperr.Wrap("IO", "无法创建素材目录", err)
		}
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := fsutil.WriteFileAtomic(filepath.Join(sourceDir, "generated.png"), input.PNG, 0o644); err != nil {
		cleanup()
		return project.Asset{}, apperr.Wrap("IO", "无法写入生成结果", err)
	}
	if len(input.BasePNG) > 0 {
		if err := fsutil.WriteFileAtomic(filepath.Join(sourceDir, "generated.base.png"), input.BasePNG, 0o644); err != nil {
			cleanup()
			return project.Asset{}, apperr.Wrap("IO", "无法写入基础层", err)
		}
	}
	if len(input.RoofPNG) > 0 {
		if err := fsutil.WriteFileAtomic(filepath.Join(sourceDir, "generated.roof.png"), input.RoofPNG, 0o644); err != nil {
			cleanup()
			return project.Asset{}, apperr.Wrap("IO", "无法写入屋顶层", err)
		}
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(versionDir, "output.png"), input.PNG, 0o644); err != nil {
		cleanup()
		return project.Asset{}, apperr.Wrap("IO", "无法写入版本图片", err)
	}
	if len(input.BasePNG) > 0 {
		if err := fsutil.WriteFileAtomic(filepath.Join(versionDir, "base.png"), input.BasePNG, 0o644); err != nil {
			cleanup()
			return project.Asset{}, apperr.Wrap("IO", "无法写入基础层版本", err)
		}
	}
	if len(input.RoofPNG) > 0 {
		if err := fsutil.WriteFileAtomic(filepath.Join(versionDir, "roof.png"), input.RoofPNG, 0o644); err != nil {
			cleanup()
			return project.Asset{}, apperr.Wrap("IO", "无法写入屋顶层版本", err)
		}
	}
	if err := writeThumbFromBytes(input.PNG, filepath.Join(dir, "thumb.png")); err != nil {
		cleanup()
		return project.Asset{}, apperr.Wrap("IO", "无法生成缩略图", err)
	}

	now := time.Now().UTC()
	sum := sha256.Sum256(input.PNG)
	hash := hex.EncodeToString(sum[:])
	meta := VersionMeta{
		Kind:      "generated",
		Width:     width,
		Height:    height,
		Hash:      hash,
		CreatedAt: now,
		App:       "pixelbench",
		Generation: &GenerationMeta{
			Generator: input.Generator,
			Seed:      input.Seed,
			Options:   options,
			Pivot:     input.Pivot,
			Collision: input.Collision,
		},
	}
	if err := fsutil.WriteJSONAtomic(filepath.Join(versionDir, "meta.json"), meta, 0o644); err != nil {
		cleanup()
		return project.Asset{}, apperr.Wrap("IO", "无法写入生成元数据", err)
	}

	name := input.Name
	if name == "" {
		name = "生成的素材"
	}
	assetType := input.Type
	if assetType == "" {
		assetType = project.AssetSprite
	}
	provider := input.Provider
	if provider == "" {
		provider = "procedural"
	}
	asset := project.Asset{
		SchemaVersion: project.SchemaVersion,
		ID:            id,
		Name:          name,
		Type:          assetType,
		Tags:          []string{"generated"},
		Status:        project.AssetDraft,
		Dimensions:    project.Dimensions{W: width, H: height},
		Pivot:         input.Pivot,
		PaletteID:     input.PaletteID,
		QuestIDs:      []string{},
		License:       "generated",
		Source: project.AssetSource{
			Kind:     "generated",
			Provider: provider,
			Workflow: input.Generator,
		},
		Versions: []project.AssetVersion{{
			ID:        "0001",
			CreatedAt: now,
			Hash:      hash,
			MetaFile:  "versions/0001/meta.json",
			Size:      int64(len(input.PNG)),
		}},
		CurrentVersion: "0001",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	saved, err := store.SaveAsset(asset)
	if err != nil {
		cleanup()
		return project.Asset{}, err
	}
	return saved, nil
}
