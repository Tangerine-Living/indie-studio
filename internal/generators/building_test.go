package generators

import (
	"bytes"
	"image/png"
	"testing"
)

func defaultOptions() BuildingOptions {
	return BuildingOptions{
		Seed:      42,
		Width:     4,
		Depth:     3,
		TileSize:  32,
		Floors:    2,
		WallColor: "#c8a87c",
		RoofColor: "#b04e3e",
		TrimColor: "#78543a",
		DoorColor: "#5c3a24",
		Sign:      true,
		Weathered: 0.4,
		Shadow:    true,
		Outline:   true,
		Variants:  1,
	}
}

func TestGenerateBuildingDeterministic(t *testing.T) {
	first, err := GenerateBuildings(defaultOptions())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := GenerateBuildings(defaultOptions())
	if err != nil {
		t.Fatalf("generate again: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("variants = %d / %d, want 1", len(first), len(second))
	}
	if !bytes.Equal(first[0].PNG, second[0].PNG) {
		t.Fatal("same seed produced different images")
	}

	img, err := png.Decode(bytes.NewReader(first[0].PNG))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Bounds().Dx() != 140 || img.Bounds().Dy() != 160 {
		t.Errorf("image = %v, want 140x160", img.Bounds())
	}
	building := first[0]
	if building.Pivot != (Point{X: 70, Y: 150}) {
		t.Errorf("pivot = %+v, want 70,150", building.Pivot)
	}
	if building.Collision != (Rect{X: 6, Y: 6, W: 128, H: 96}) {
		t.Errorf("collision = %+v", building.Collision)
	}
	r, _, _, a := img.At(70, 140).RGBA()
	if a == 0 {
		t.Error("wall pixel is transparent")
	}
	if r == 0 {
		t.Error("wall pixel has no colour")
	}
}

func TestGenerateBuildingVariantsDiffer(t *testing.T) {
	opts := defaultOptions()
	opts.Variants = 4
	results, err := GenerateBuildings(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("variants = %d, want 4", len(results))
	}
	seen := map[string]bool{}
	for _, item := range results {
		key := string(item.PNG)
		if seen[key] {
			t.Fatalf("seed %d repeated an earlier variant", item.Seed)
		}
		seen[key] = true
	}
	if results[0].Seed != opts.Seed {
		t.Errorf("first seed = %d, want %d", results[0].Seed, opts.Seed)
	}
}

func TestGenerateBuildingClampsOptions(t *testing.T) {
	opts := defaultOptions()
	opts.Width = 0
	opts.Depth = 99
	opts.Floors = 0
	opts.TileSize = 33
	opts.Variants = 99
	opts.Weathered = 2
	results, err := GenerateBuildings(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(results) != maxVariants {
		t.Errorf("variants = %d, want %d", len(results), maxVariants)
	}
	for _, item := range results {
		img, err := png.Decode(bytes.NewReader(item.PNG))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		// width 1, depth 16, tile 32, floors 1 -> 32x(512+32) with padding.
		if img.Bounds().Dx() != 44 {
			t.Errorf("width = %d, want 44", img.Bounds().Dx())
		}
	}
}
