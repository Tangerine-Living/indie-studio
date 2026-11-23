package assets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"pixelbench/internal/imageproc"
	"pixelbench/internal/project"
)

func TestSavePostProcessedVersionWritesChainMeta(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)

	saved, result, err := SaveProcessedAsset(store, asset.ID, imageproc.Options{
		Width:     8,
		Height:    8,
		MaxColors: 4,
		Colors:    []string{"#000000", "#ffffff", "#cc3333", "#3c3cdc"},
		Outline:   true,
		CheckSeam: true,
	})
	if err != nil {
		t.Fatalf("save post-processed: %v", err)
	}
	// The seeded art is full-bleed, so the outline grows the canvas by 1px on
	// each side instead of clipping the border.
	if result.Width != 10 || result.Height != 10 {
		t.Fatalf("result = %dx%d", result.Width, result.Height)
	}
	if len(result.Colors) != 4 {
		t.Errorf("colors = %v", result.Colors)
	}
	if saved.CurrentVersion == "" || len(saved.Versions) != 1 {
		t.Fatalf("versions = %+v current=%q", saved.Versions, saved.CurrentVersion)
	}
	if saved.Dimensions != (project.Dimensions{W: 10, H: 10}) {
		t.Errorf("dimensions = %+v", saved.Dimensions)
	}
	// The source pivot (16,16 on 32×32) maps to the new centre, shifted by the
	// outline padding.
	if saved.Pivot != (project.Point{X: 5, Y: 5}) {
		t.Errorf("pivot = %+v, want 5,5", saved.Pivot)
	}

	dir := project.AssetDir(store.Root(), asset.ID)
	versionDir := filepath.Join(dir, "versions", saved.CurrentVersion)
	if _, err := os.Stat(filepath.Join(versionDir, "output.png")); err != nil {
		t.Fatalf("output.png missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "thumb.png")); err != nil {
		t.Errorf("thumb.png missing: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(versionDir, "meta.json"))
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var meta struct {
		Kind    string `json:"kind"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
		Process struct {
			Palette []string `json:"palette"`
			Options struct {
				Dither string `json:"dither"`
			} `json:"options"`
			Seam struct {
				LeftRight float64 `json:"leftRight"`
				TopBottom float64 `json:"topBottom"`
			} `json:"seam"`
		} `json:"process"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if meta.Kind != "postprocess" {
		t.Errorf("kind = %q", meta.Kind)
	}
	if meta.Width != 10 || meta.Height != 10 {
		t.Errorf("meta size = %dx%d", meta.Width, meta.Height)
	}
	if len(meta.Process.Palette) != 4 {
		t.Errorf("meta palette = %v", meta.Process.Palette)
	}
	if meta.Process.Options.Dither != "none" {
		t.Errorf("meta dither = %q", meta.Process.Options.Dither)
	}
}

func TestProcessAssetMissingSource(t *testing.T) {
	store := newProject(t)
	if _, err := ProcessAsset(store, "missing", imageproc.Options{}); err == nil {
		t.Fatal("expected an error for a missing asset")
	}
}
