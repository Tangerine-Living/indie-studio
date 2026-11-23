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

// newProject creates a project under a Chinese path with spaces, which also
// covers the M0 acceptance case for non-ASCII Windows paths.
func newProject(t *testing.T) *project.Store {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "中文 空格")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	store, err := project.Create(project.CreateOptions{
		ParentDir: parent,
		Name:      "小镇 广场",
		Engine:    "godot",
		TileSize:  32,
		Template:  "town-square",
		PaletteID: "default",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func writePNG(t *testing.T, dir, name string, w, h int, c color.NRGBA) string {
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
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	return path
}

func thumbSize(t *testing.T, path string) (int, int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read thumb: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode thumb: %v", err)
	}
	return cfg.Width, cfg.Height
}

func TestImportCopiesHashesAndThumbs(t *testing.T) {
	store := newProject(t)
	root := store.Root()
	input := t.TempDir()

	small := writePNG(t, input, "草地.png", 32, 32, color.NRGBA{R: 0x4f, G: 0x8f, B: 0x4a, A: 0xff})
	large := writePNG(t, input, "杂货铺.png", 64, 48, color.NRGBA{R: 0xb8, G: 0x5c, B: 0x38, A: 0xff})
	copyOfSmall := writePNG(t, input, "草地 副本.png", 32, 32, color.NRGBA{R: 0x4f, G: 0x8f, B: 0x4a, A: 0xff})
	notes := filepath.Join(input, "说明.txt")
	if err := os.WriteFile(notes, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := Import(store, []string{small, large, copyOfSmall, notes}, project.AssetTile)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(summary.Imported) != 2 {
		t.Fatalf("imported = %d, want 2", len(summary.Imported))
	}
	if len(summary.Duplicates) != 1 {
		t.Fatalf("duplicates = %d, want 1", len(summary.Duplicates))
	}
	if len(summary.Failed) != 1 {
		t.Fatalf("failed = %d, want 1", len(summary.Failed))
	}

	smallAsset := summary.Imported[0]
	if smallAsset.Type != project.AssetTile {
		t.Errorf("type = %q, want tile", smallAsset.Type)
	}
	if smallAsset.Name != "草地" {
		t.Errorf("name = %q, want 草地", smallAsset.Name)
	}
	if smallAsset.Dimensions.W != 32 || smallAsset.Dimensions.H != 32 {
		t.Errorf("dimensions = %+v, want 32x32", smallAsset.Dimensions)
	}
	if smallAsset.SourceHash == "" {
		t.Error("source hash is empty")
	}
	if smallAsset.Source.Kind != "imported" {
		t.Errorf("source kind = %q", smallAsset.Source.Kind)
	}
	if smallAsset.Status != project.AssetDraft {
		t.Errorf("status = %q, want draft", smallAsset.Status)
	}

	assetDir := project.AssetDir(root, smallAsset.ID)
	if _, err := os.Stat(filepath.Join(assetDir, "source", "草地.png")); err != nil {
		t.Errorf("original not kept: %v", err)
	}
	if w, h := thumbSize(t, filepath.Join(assetDir, "thumb.png")); w != 128 || h != 128 {
		t.Errorf("small thumb = %dx%d, want 128x128", w, h)
	}
	largeDir := project.AssetDir(root, summary.Imported[1].ID)
	if w, h := thumbSize(t, filepath.Join(largeDir, "thumb.png")); w != 256 || h != 192 {
		t.Errorf("large thumb = %dx%d, want 256x192", w, h)
	}

	// Reopen the project: manifests and hashes must survive a restart and still
	// deduplicate.
	path := store.Root()
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	reopened, err := project.Open(path, false)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	assets, err := reopened.ListAssets()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(assets) != 4 {
		t.Fatalf("assets after reopen = %d, want 4 (seeds + 2)", len(assets))
	}
	found := false
	for _, asset := range assets {
		if asset.ID == smallAsset.ID {
			found = true
			if asset.SourceHash != smallAsset.SourceHash {
				t.Errorf("hash not persisted: %q vs %q", asset.SourceHash, smallAsset.SourceHash)
			}
		}
	}
	if !found {
		t.Fatal("imported asset missing after reopen")
	}

	again, err := Import(reopened, []string{small}, project.AssetTile)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if len(again.Imported) != 0 || len(again.Duplicates) != 1 {
		t.Errorf("re-import imported=%d duplicates=%d, want 0/1", len(again.Imported), len(again.Duplicates))
	}
}

func TestImportRejectsUnknownType(t *testing.T) {
	store := newProject(t)
	_, err := Import(store, []string{"C:\\nope.png"}, "bogus")
	if !apperr.Is(err, "INVALID_ASSET") {
		t.Fatalf("err = %v, want INVALID_ASSET", err)
	}
}

func TestImportNeedsPaths(t *testing.T) {
	store := newProject(t)
	_, err := Import(store, nil, project.AssetSprite)
	if !apperr.Is(err, "INVALID_IMPORT") {
		t.Fatalf("err = %v, want INVALID_IMPORT", err)
	}
}
