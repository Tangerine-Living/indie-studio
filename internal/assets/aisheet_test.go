package assets

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"pixelbench/internal/project"
)

func aiGrid(t *testing.T, columns, rows, cell int, colors []color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, columns*cell, rows*cell))
	for index, c := range colors {
		col, row := index%columns, index/columns
		// A 12px block sitting on the bottom edge leaves room for trim/align.
		x0 := col*cell + (cell-12)/2
		y0 := row*cell + cell - 12
		for y := 0; y < 12; y++ {
			for x := 0; x < 12; x++ {
				img.SetNRGBA(x0+x, y0+y, c)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode grid: %v", err)
	}
	return buf.Bytes()
}

func TestCreateAISheetSlicesAndAligns(t *testing.T) {
	store := newProject(t)
	colors := []color.NRGBA{
		{R: 255, A: 255},
		{G: 255, A: 255},
		{B: 255, A: 255},
		{R: 255, G: 255, A: 255},
	}
	data := aiGrid(t, 4, 1, 32, colors)
	asset, err := CreateAISheet(store, data, AISheetOptions{
		Name:          "四向测试",
		AssetType:     project.AssetSprite,
		Columns:       4,
		Rows:          1,
		FrameSize:     16,
		FPS:           8,
		Tags:          []string{"walk"},
		PaletteID:     "default",
		KeyBackground: true,
	})
	if err != nil {
		t.Fatalf("CreateAISheet: %v", err)
	}
	if asset.Animation == nil {
		t.Fatal("expected animation metadata")
	}
	anim := asset.Animation
	if anim.FrameCount != 4 || anim.Columns != 4 || anim.FPS != 8 {
		t.Fatalf("animation = %+v", anim)
	}
	if anim.FrameWidth != 18 || anim.FrameHeight != 18 {
		t.Fatalf("frame size = %d×%d, want 18×18", anim.FrameWidth, anim.FrameHeight)
	}
	if asset.Dimensions.W != 18 || asset.Dimensions.H != 18 {
		t.Fatalf("asset dimensions = %+v", asset.Dimensions)
	}
	if len(asset.Versions) != 1 {
		t.Fatalf("versions = %d, want 1", len(asset.Versions))
	}
	frames, err := os.ReadDir(filepath.Join(project.AssetDir(store.Root(), asset.ID), "frames"))
	if err != nil {
		t.Fatalf("read frames: %v", err)
	}
	if len(frames) != 4 {
		t.Fatalf("frames = %d, want 4", len(frames))
	}
	if _, err := store.GetAsset(asset.ID); err != nil {
		t.Fatalf("asset not listed: %v", err)
	}
}

// Frames must quantise against one shared palette: separate per-frame
// palettes make colours pop between frames even when the prompt is consistent.
func TestCreateAISheetSharesPalette(t *testing.T) {
	store := newProject(t)
	cell := 64
	img := image.NewNRGBA(image.Rect(0, 0, 2*cell, cell))
	paint := func(column int, colors []color.NRGBA) {
		for y := 0; y < cell; y++ {
			for x := 0; x < cell; x++ {
				stripe := color.NRGBA{}
				switch {
				case x < cell/3:
					stripe = colors[0]
				case x < 2*cell/3:
					stripe = colors[1]
				default:
					stripe = colors[2]
				}
				img.SetNRGBA(column*cell+x, y, stripe)
			}
		}
	}
	paint(0, []color.NRGBA{
		{R: 240, A: 255},
		{G: 240, A: 255},
		{B: 240, A: 255},
	})
	paint(1, []color.NRGBA{
		{R: 240, G: 240, A: 255},
		{G: 240, B: 240, A: 255},
		{R: 240, B: 240, A: 255},
	})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode sheet: %v", err)
	}

	asset, err := CreateAISheet(store, buf.Bytes(), AISheetOptions{
		Name:      "共享调色板",
		Columns:   2,
		Rows:      1,
		FrameSize: 32,
		MaxColors: 2,
	})
	if err != nil {
		t.Fatalf("CreateAISheet: %v", err)
	}

	framesDir := filepath.Join(project.AssetDir(store.Root(), asset.ID), "frames")
	if _, err := os.Stat(framesDir); err != nil {
		t.Fatalf("frames dir: %v", err)
	}
	frameColors := func(index int) map[color.NRGBA]bool {
		raw, err := os.ReadFile(filepath.Join(framesDir, fmt.Sprintf("%03d.png", index)))
		if err != nil {
			t.Fatalf("read frame %d: %v", index, err)
		}
		frame, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("decode frame %d: %v", index, err)
		}
		seen := map[color.NRGBA]bool{}
		bounds := frame.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, a := frame.At(x, y).RGBA()
				if a == 0 {
					continue
				}
				seen[color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}] = true
			}
		}
		return seen
	}
	first, second := frameColors(0), frameColors(1)
	if len(first) == 0 || len(first) > 2 {
		t.Fatalf("frame 0 colours = %d, want 1..2", len(first))
	}
	if len(second) == 0 || len(second) > 2 {
		t.Fatalf("frame 1 colours = %d, want 1..2", len(second))
	}
	for c := range first {
		if !second[c] {
			t.Fatalf("frame colours differ: %v vs %v", first, second)
		}
	}
}

func TestCreateAISheetRejectsBadGrid(t *testing.T) {
	store := newProject(t)
	data := aiGrid(t, 2, 1, 8, []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}})
	if _, err := CreateAISheet(store, data, AISheetOptions{
		Columns:   1,
		Rows:      1,
		FrameSize: 16,
	}); err != nil {
		t.Fatalf("single cell should work: %v", err)
	}
	if _, err := CreateAISheet(store, data, AISheetOptions{
		Columns:   4,
		Rows:      4,
		FrameSize: 16,
	}); err == nil {
		t.Fatal("expected too-small grid to fail")
	}
}