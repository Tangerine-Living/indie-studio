package assets

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

func TestExportAssetCopiesCurrentVersion(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	saved, err := SaveVersion(store, asset.ID, encodePNG(t, 8, 8, color.NRGBA{R: 0x6e, G: 0xc6, B: 0xe8, A: 0xff}))
	if err != nil {
		t.Fatalf("save version: %v", err)
	}

	result, err := ExportAsset(store, saved.ID)
	if err != nil {
		t.Fatalf("export asset: %v", err)
	}
	wantDir := filepath.Join(project.ExportsDir(store.Root()), sanitizeExportName(saved.Name), saved.CurrentVersion)
	if result.Dir != wantDir {
		t.Errorf("dir = %q, want %q", result.Dir, wantDir)
	}
	for _, name := range []string{"output.png", "meta.json", "export.json"} {
		if _, statErr := os.Stat(filepath.Join(wantDir, name)); statErr != nil {
			t.Errorf("missing %s: %v", name, statErr)
		}
	}

	var marker exportMarker
	if err := fsutil.ReadJSON(filepath.Join(wantDir, "export.json"), &marker); err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker.AssetID != saved.ID || marker.Version != saved.CurrentVersion || marker.AssetName != saved.Name {
		t.Errorf("marker = %+v", marker)
	}

	// Re-exporting the same version overwrites in place and stays valid.
	if _, err := ExportAsset(store, saved.ID); err != nil {
		t.Fatalf("re-export: %v", err)
	}
}

func TestExportAssetIncludesSheetMetadataAndFrames(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	info := sheetInfo()
	info.Durations = []int{60, 140}
	info.Tags = []string{"walk"}
	saved, err := SaveSheet(store, asset.ID, encodeSheet(t, info), info)
	if err != nil {
		t.Fatalf("save sheet: %v", err)
	}

	result, err := ExportAsset(store, saved.ID)
	if err != nil {
		t.Fatalf("export sheet: %v", err)
	}
	for _, rel := range []string{
		"output.png",
		"meta.json",
		"frames-aseprite.json",
		"frames-texturepacker.json",
		"frames/000.png",
		"frames/001.png",
		"export.json",
	} {
		if _, statErr := os.Stat(filepath.Join(result.Dir, filepath.FromSlash(rel))); statErr != nil {
			t.Errorf("missing %s: %v", rel, statErr)
		}
	}

	raw, err := os.ReadFile(filepath.Join(result.Dir, "frames-aseprite.json"))
	if err != nil {
		t.Fatalf("read aseprite json: %v", err)
	}
	var payload struct {
		Frames map[string]struct {
			Duration int `json:"duration"`
		} `json:"frames"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse aseprite json: %v", err)
	}
	if payload.Frames["001"].Duration != 140 {
		t.Errorf("exported frame 001 duration = %d, want 140", payload.Frames["001"].Duration)
	}
}

func TestExportAssetRequiresVersion(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	if _, err := ExportAsset(store, asset.ID); !apperr.Is(err, "NO_VERSION") {
		t.Fatalf("error = %v, want NO_VERSION", err)
	}
	if _, err := ExportAsset(store, "missing"); err == nil {
		t.Fatal("expected missing asset error")
	}
}

func TestExportFolderNameAvoidsSharedNames(t *testing.T) {
	root := t.TempDir()
	first := project.Asset{ID: "asset-one", Name: "杂货铺"}
	name := exportFolderName(root, first)
	if name != "杂货铺" {
		t.Errorf("folder = %q, want 杂货铺", name)
	}
	if err := fsutil.WriteJSONAtomic(filepath.Join(root, name, "export.json"), exportMarker{
		AssetID:   first.ID,
		AssetName: first.Name,
		Version:   "0001",
	}, 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	second := project.Asset{ID: "asset-two", Name: "杂货铺"}
	if got := exportFolderName(root, second); got != "杂货铺-assettwo" {
		t.Errorf("collision folder = %q, want 杂货铺-assettwo", got)
	}
}

func TestSanitizeExportName(t *testing.T) {
	cases := map[string]string{
		"":         "asset",
		"  ":       "asset",
		"CON":      "_CON",
		"a/b:c*d":  "a_b_c_d",
		"..":       "asset",
		"杂货铺 ... ": "杂货铺",
	}
	for input, want := range cases {
		if got := sanitizeExportName(input); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", input, got, want)
		}
	}
}
