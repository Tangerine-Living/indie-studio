package imageproc

import (
	"image"
	"image/color"
	"testing"
)

func newImage(w, h int, fill color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	return img
}

func set(img *image.NRGBA, x, y int, c color.NRGBA) {
	img.SetNRGBA(x, y, c)
}

var (
	red   = color.NRGBA{R: 220, G: 40, B: 40, A: 255}
	blue  = color.NRGBA{R: 40, G: 60, B: 220, A: 255}
	green = color.NRGBA{R: 40, G: 200, B: 80, A: 255}
)

func TestProcessTrimsAndResizes(t *testing.T) {
	src := newImage(16, 16, color.NRGBA{})
	for y := 4; y < 8; y++ {
		for x := 4; x < 8; x++ {
			set(src, x, y, red)
		}
	}

	result := Process(src, Options{
		Trim:      true,
		Width:     4,
		Height:    4,
		Colors:    []string{"#dc2828"},
		MaxColors: 4,
	})
	if result.Width != 4 || result.Height != 4 {
		t.Fatalf("size = %dx%d", result.Width, result.Height)
	}
	if got := result.Image.NRGBAAt(0, 0); got.A != 255 || got.R < 200 {
		t.Errorf("trimmed pixel = %+v", got)
	}
	if result.Colors[0] != "#dc2828" {
		t.Errorf("colors = %v", result.Colors)
	}
	if src.NRGBAAt(0, 0).A != 0 {
		t.Error("source image was mutated")
	}
}

func TestQuantizeSnapsToPalette(t *testing.T) {
	src := newImage(4, 4, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	set(src, 0, 0, color.NRGBA{R: 210, G: 60, B: 60, A: 255})

	result := Process(src, Options{
		MaxColors: 8,
		Colors:    []string{"#000000", "#ffffff", "#cc3333"},
	})
	allowed := map[uint32]bool{}
	for _, hex := range result.Colors {
		c := parseColor(hex, color.NRGBA{})
		allowed[uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B)] = true
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := result.Image.NRGBAAt(x, y)
			key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
			if !allowed[key] {
				t.Fatalf("pixel (%d,%d) = %+v not in palette", x, y, c)
			}
		}
	}
	if got := result.Image.NRGBAAt(0, 0); got.R < 150 {
		t.Errorf("red-ish pixel quantised to %+v", got)
	}
}

func TestDitherModesAreDeterministicAndDiffer(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			shade := uint8(20 + x*12)
			src.SetNRGBA(x, y, color.NRGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	base := Options{Colors: []string{"#111111", "#eeeeee"}, DitherStrength: 1}
	variants := map[DitherMode]*image.NRGBA{}
	for _, mode := range []DitherMode{DitherNone, DitherFloyd, DitherBayer, DitherNoise} {
		opts := base
		opts.Dither = mode
		first := Process(src, opts)
		second := Process(src, opts)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if first.Image.NRGBAAt(x, y) != second.Image.NRGBAAt(x, y) {
					t.Fatalf("%s is not deterministic at (%d,%d)", mode, x, y)
				}
			}
		}
		variants[mode] = first.Image
	}

	differ := func(a, b *image.NRGBA) bool {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if a.NRGBAAt(x, y) != b.NRGBAAt(x, y) {
					return true
				}
			}
		}
		return false
	}
	for _, mode := range []DitherMode{DitherFloyd, DitherBayer, DitherNoise} {
		if !differ(variants[DitherNone], variants[mode]) {
			t.Errorf("%s produced the same image as no dithering", mode)
		}
	}
}

func TestCleanupAndOutline(t *testing.T) {
	src := newImage(10, 10, color.NRGBA{})
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			set(src, x, y, green)
		}
	}
	set(src, 8, 8, green) // isolated speck

	cleaned := Process(src, Options{Colors: []string{"#28c850"}, Cleanup: true})
	if cleaned.Image.NRGBAAt(8, 8).A != 0 {
		t.Error("isolated pixel was not removed")
	}
	if cleaned.Image.NRGBAAt(3, 3).A == 0 {
		t.Error("block pixel was removed")
	}

	outlined := Process(src, Options{
		Colors:       []string{"#28c850"},
		Outline:      true,
		OutlineColor: "#0c1016",
	})
	if got := outlined.Image.NRGBAAt(1, 3); got.A != 255 || got.R > 20 {
		t.Errorf("outline pixel = %+v", got)
	}
	if got := outlined.Image.NRGBAAt(3, 3); got.G < 100 {
		t.Errorf("body pixel overwritten: %+v", got)
	}
}

func TestOutlinePadsClippedArt(t *testing.T) {
	// Full-bleed art would lose its border on every side; the canvas grows.
	src := newImage(4, 4, red)
	result := Process(src, Options{
		Colors:       []string{"#dc2828"},
		Outline:      true,
		OutlineColor: "#0c1016",
	})
	if result.Width != 6 || result.Height != 6 {
		t.Fatalf("size = %dx%d, want 6x6", result.Width, result.Height)
	}
	if got := result.Image.NRGBAAt(1, 0); got.R != 12 || got.G != 16 || got.B != 22 || got.A != 255 {
		t.Errorf("top outline = %+v", got)
	}
	if got := result.Image.NRGBAAt(0, 1); got.A != 255 {
		t.Errorf("left outline = %+v", got)
	}
	if got := result.Image.NRGBAAt(1, 1); got.R < 200 || got.A != 255 {
		t.Errorf("art shifted incorrectly: %+v", got)
	}
	if got := result.Image.NRGBAAt(4, 5); got.A == 0 {
		t.Error("outline missing at far edge")
	}
}

func TestOutlineUsesDarkestPaletteColor(t *testing.T) {
	src := newImage(4, 4, color.NRGBA{R: 64, G: 64, B: 64, A: 255})
	result := Process(src, Options{
		Colors:  []string{"#ffffff", "#101820"},
		Outline: true, // no explicit colour: auto from the palette
	})
	dark := color.NRGBA{R: 16, G: 24, B: 32, A: 255}
	if got := result.Image.NRGBAAt(1, 0); got != dark {
		t.Errorf("auto outline = %+v, want %+v", got, dark)
	}
	count := 0
	for _, hex := range result.Colors {
		if hex == "#101820" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("palette colours = %v, want #101820 once", result.Colors)
	}
}

func TestExtractPaletteForSharesColors(t *testing.T) {
	first := newImage(2, 2, red)
	second := newImage(2, 2, blue)
	colors := ExtractPaletteFor([]*image.NRGBA{first, second}, 4)
	if len(colors) != 2 {
		t.Fatalf("colors = %v, want both source colours", colors)
	}
	if colors[0] != hexOf(red) && colors[1] != hexOf(red) {
		t.Errorf("red missing from %v", colors)
	}
	if got := ExtractPaletteFor([]*image.NRGBA{first, second}, 1); len(got) != 1 {
		t.Errorf("clamped palette = %v, want one colour", got)
	}
}

func TestSeamReport(t *testing.T) {
	broken := newImage(8, 8, red)
	for y := 0; y < 8; y++ {
		set(broken, 7, y, blue)
	}
	report := Process(broken, Options{Colors: []string{"#dc2828", "#3c3cdc"}, CheckSeam: true}).Seam
	if report.LeftRight <= 0 {
		t.Errorf("left/right seam = %v", report.LeftRight)
	}

	uniform := Process(newImage(8, 8, red), Options{Colors: []string{"#dc2828"}, CheckSeam: true}).Seam
	if uniform.LeftRight != 0 || uniform.TopBottom != 0 {
		t.Errorf("uniform seam = %+v", uniform)
	}
}

func TestAutoPaletteRespectsMaxColors(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 8), G: uint8(y * 8), B: 128, A: 255})
		}
	}
	result := Process(src, Options{MaxColors: 4})
	if len(result.Colors) == 0 || len(result.Colors) > 4 {
		t.Fatalf("palette = %v", result.Colors)
	}
	if len(result.Image.Pix) == 0 {
		t.Fatal("no output image")
	}
}

func TestDownscaleAveragesSoftAlphaBeforeThresholding(t *testing.T) {
	// Half the pixels are solid and half are faint (alpha 100). Their combined
	// coverage (about 69%) must survive; binarising before the box filter would
	// count only 50% and drop the pixel.
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			alpha := uint8(100)
			if (x+y)%2 == 0 {
				alpha = 255
			}
			src.SetNRGBA(x, y, color.NRGBA{R: 240, G: 240, B: 240, A: alpha})
		}
	}
	result := Process(src, Options{
		Width:          1,
		Height:         1,
		Colors:         []string{"#f0f0f0"},
		AlphaThreshold: 128,
	})
	if got := result.Image.NRGBAAt(0, 0); got.A != 255 {
		t.Errorf("coverage pixel = %+v, want opaque", got)
	}
}

func TestAlphaThresholdBinarises(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 40})
	src.SetNRGBA(1, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 200})
	src.SetNRGBA(2, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 127})
	src.SetNRGBA(3, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 128})

	result := Process(src, Options{Colors: []string{"#0a141e"}, AlphaThreshold: 128})
	want := []uint8{0, 255, 0, 255}
	for x, alpha := range want {
		if got := result.Image.NRGBAAt(x, 0).A; got != alpha {
			t.Errorf("alpha[%d] = %d, want %d", x, got, alpha)
		}
	}
}
