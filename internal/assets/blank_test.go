package assets

import (
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/project"
)

func TestCreateBlankAsset(t *testing.T) {
	store := newProject(t)

	asset, err := CreateBlank(store, BlankAssetInput{
		Name:   " 手绘主角 ",
		Type:   project.AssetSprite,
		Width:  24,
		Height: 16,
	})
	if err != nil {
		t.Fatalf("CreateBlank: %v", err)
	}
	if asset.Name != "手绘主角" {
		t.Fatalf("name = %q", asset.Name)
	}
	if asset.Type != project.AssetSprite {
		t.Fatalf("type = %q", asset.Type)
	}
	if asset.Status != project.AssetDraft {
		t.Fatalf("status = %q", asset.Status)
	}
	if asset.Dimensions != (project.Dimensions{W: 24, H: 16}) {
		t.Fatalf("dimensions = %+v", asset.Dimensions)
	}
	if asset.Pivot != (project.Point{X: 12, Y: 15}) {
		t.Fatalf("pivot = %+v", asset.Pivot)
	}
	if asset.Source.Kind != "blank" {
		t.Fatalf("source = %+v", asset.Source)
	}
	if len(asset.Versions) != 0 {
		t.Fatalf("versions = %d, want 0", len(asset.Versions))
	}
	if asset.PaletteID != "default" {
		t.Fatalf("palette = %q", asset.PaletteID)
	}

	list, err := store.ListAssets()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range list {
		if item.ID == asset.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("blank asset missing from the library")
	}
}

func TestCreateBlankAssetRejectsBadInput(t *testing.T) {
	store := newProject(t)
	cases := []BlankAssetInput{
		{Name: "   ", Type: project.AssetSprite, Width: 16, Height: 16},
		{Name: "非法类型", Type: "boss", Width: 16, Height: 16},
		{Name: "零尺寸", Type: project.AssetSprite, Width: 0, Height: 16},
		{Name: "太大", Type: project.AssetSprite, Width: 2049, Height: 16},
	}
	for _, input := range cases {
		if _, err := CreateBlank(store, input); !apperr.Is(err, "INVALID_ASSET") {
			t.Fatalf("input %+v error = %v, want INVALID_ASSET", input, err)
		}
	}
}