package assets

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/project"
)

func encodePNG(t *testing.T, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func seededAsset(t *testing.T, store *project.Store) project.Asset {
	t.Helper()
	assets, err := store.ListAssets()
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) == 0 {
		t.Fatal("project has no seeded asset")
	}
	return assets[0]
}

func TestSaveVersionWritesImmutableOutput(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	root := store.Root()

	saved, err := SaveVersion(store, asset.ID, encodePNG(t, 16, 16, color.NRGBA{R: 0x6e, G: 0xc6, B: 0xe8, A: 0xff}))
	if err != nil {
		t.Fatalf("save version: %v", err)
	}
	if saved.CurrentVersion != "0001" {
		t.Errorf("current version = %q, want 0001", saved.CurrentVersion)
	}
	if len(saved.Versions) != 1 {
		t.Fatalf("versions = %d, want 1", len(saved.Versions))
	}
	if saved.Dimensions.W != 16 || saved.Dimensions.H != 16 {
		t.Errorf("dimensions = %+v, want 16x16", saved.Dimensions)
	}
	version := saved.Versions[0]
	if version.MetaFile != "versions/0001/meta.json" {
		t.Errorf("meta file = %q", version.MetaFile)
	}
	if len(version.Hash) != 64 {
		t.Errorf("hash = %q, want sha256 hex", version.Hash)
	}

	dir := filepath.Join(project.AssetDir(root, asset.ID), "versions", "0001")
	if _, err := os.Stat(filepath.Join(dir, "output.png")); err != nil {
		t.Errorf("output.png missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "meta.json")); err != nil {
		t.Errorf("meta.json missing: %v", err)
	}
	if w, h := thumbSize(t, filepath.Join(project.AssetDir(root, asset.ID), "thumb.png")); w != 64 || h != 64 {
		t.Errorf("thumb = %dx%d, want 64x64", w, h)
	}

	second, err := SaveVersion(store, asset.ID, encodePNG(t, 8, 8, color.NRGBA{R: 0xf2, G: 0xb8, B: 0x4b, A: 0xff}))
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if second.CurrentVersion != "0002" || len(second.Versions) != 2 {
		t.Errorf("second save current=%q versions=%d", second.CurrentVersion, len(second.Versions))
	}

	path := store.Root()
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	reopened, err := project.Open(path, false)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	reloaded, err := reopened.GetAsset(asset.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if reloaded.CurrentVersion != "0002" || len(reloaded.Versions) != 2 {
		t.Errorf("after reopen current=%q versions=%d", reloaded.CurrentVersion, len(reloaded.Versions))
	}
}

func TestSaveVersionRejectsBadData(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)

	if _, err := SaveVersion(store, asset.ID, nil); !apperr.Is(err, "INVALID_VERSION") {
		t.Errorf("empty data err = %v, want INVALID_VERSION", err)
	}
	if _, err := SaveVersion(store, asset.ID, []byte("not a png")); !apperr.Is(err, "INVALID_VERSION") {
		t.Errorf("garbage err = %v, want INVALID_VERSION", err)
	}
	if _, err := SaveVersion(store, "missing", encodePNG(t, 4, 4, color.NRGBA{A: 0xff})); !apperr.Is(err, "ASSET_NOT_FOUND") {
		t.Errorf("missing asset err = %v, want ASSET_NOT_FOUND", err)
	}
}
