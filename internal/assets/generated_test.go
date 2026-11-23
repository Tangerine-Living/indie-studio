package assets

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/project"
)

func generatedInput(t *testing.T) GeneratedInput {
	t.Helper()
	png := encodePNG(t, 12, 10, color.NRGBA{R: 0xb0, G: 0x4e, B: 0x3e, A: 0xff})
	return GeneratedInput{
		Name:      "杂货铺",
		Type:      project.AssetBuilding,
		PaletteID: "default",
		Width:     12,
		Height:    10,
		Pivot:     project.Point{X: 6, Y: 9},
		PNG:       png,
		BasePNG:   encodePNG(t, 12, 10, color.NRGBA{R: 0xc8, A: 0xff}),
		RoofPNG:   encodePNG(t, 12, 10, color.NRGBA{B: 0x90, A: 0xff}),
		Generator: "building-v1",
		Seed:      1234,
		Options:   map[string]any{"width": 4, "depth": 3, "floors": 2},
		Collision: AnimationRect{X: 1, Y: 1, W: 10, H: 6},
	}
}

func TestCreateGeneratedAssetWritesLayersAndMeta(t *testing.T) {
	store := newProject(t)
	input := generatedInput(t)

	saved, err := CreateGeneratedAsset(store, input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if saved.Name != "杂货铺" || saved.Type != project.AssetBuilding {
		t.Errorf("asset = %+v", saved)
	}
	if saved.Source.Kind != "generated" || saved.Source.Workflow != "building-v1" {
		t.Errorf("source = %+v", saved.Source)
	}
	if saved.Dimensions != (project.Dimensions{W: 12, H: 10}) {
		t.Errorf("dimensions = %+v", saved.Dimensions)
	}
	if saved.Pivot != (project.Point{X: 6, Y: 9}) {
		t.Errorf("pivot = %+v", saved.Pivot)
	}
	if saved.CurrentVersion != "0001" {
		t.Errorf("version = %q", saved.CurrentVersion)
	}

	dir := project.AssetDir(store.Root(), saved.ID)
	for _, rel := range []string{
		"asset.json",
		"source/generated.png",
		"source/generated.base.png",
		"source/generated.roof.png",
		"versions/0001/output.png",
		"versions/0001/base.png",
		"versions/0001/roof.png",
		"versions/0001/meta.json",
		"thumb.png",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, "versions/0001/meta.json"))
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var meta struct {
		Kind       string `json:"kind"`
		Generation struct {
			Generator string          `json:"generator"`
			Seed      int64           `json:"seed"`
			Options   json.RawMessage `json:"options"`
			Collision AnimationRect   `json:"collision"`
		} `json:"generation"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if meta.Kind != "generated" {
		t.Errorf("meta kind = %q", meta.Kind)
	}
	if meta.Generation.Seed != 1234 || meta.Generation.Generator != "building-v1" {
		t.Errorf("generation = %+v", meta.Generation)
	}
	if meta.Generation.Collision != input.Collision {
		t.Errorf("collision = %+v", meta.Generation.Collision)
	}
	var options map[string]any
	if err := json.Unmarshal(meta.Generation.Options, &options); err != nil {
		t.Fatalf("decode options: %v", err)
	}
	if options["floors"] != float64(2) {
		t.Errorf("options = %v", options)
	}

	assets, err := store.ListAssets()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, item := range assets {
		if item.ID == saved.ID {
			found = true
		}
	}
	if !found {
		t.Error("generated asset missing from list")
	}
}

func TestCreateGeneratedAssetRejectsBadInput(t *testing.T) {
	store := newProject(t)

	if _, err := CreateGeneratedAsset(store, GeneratedInput{}); !apperr.Is(err, "INVALID_GENERATED") {
		t.Errorf("empty payload error = %v", err)
	}

	input := generatedInput(t)
	input.Width = 99
	if _, err := CreateGeneratedAsset(store, input); !apperr.Is(err, "INVALID_GENERATED") {
		t.Errorf("size mismatch error = %v", err)
	}

	input = generatedInput(t)
	input.RoofPNG = encodePNG(t, 4, 4, color.NRGBA{A: 0xff})
	if _, err := CreateGeneratedAsset(store, input); !apperr.Is(err, "INVALID_GENERATED") {
		t.Errorf("layer mismatch error = %v", err)
	}
}
