// Package imageproc turns AI or imported artwork into clean, palette-locked
// pixel art. The chain is deliberately explicit and deterministic: trim,
// box-downsample, OKLab quantisation (optional dither), despeckle, 1px outline
// and a tile-seam report. Nothing here depends on wall-clock or randomness.
package imageproc

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"
)

// DitherMode selects how quantisation error is spread.
type DitherMode string

const (
	DitherNone  DitherMode = "none"
	DitherFloyd DitherMode = "floyd"
	DitherBayer DitherMode = "bayer"
	DitherNoise DitherMode = "noise"
)

// Options are the post-processing knobs exposed to the UI.
type Options struct {
	// Width/Height are the target pixel size. Leave both at 0 to keep the
	// (trimmed) source size; set only one to preserve the aspect ratio.
	Width  int `json:"width"`
	Height int `json:"height"`
	// Trim crops transparent borders before scaling.
	Trim bool `json:"trim"`
	// AlphaThreshold splits translucency into fully opaque/transparent.
	AlphaThreshold int `json:"alphaThreshold"`
	// MaxColors is used when Colors is empty (auto palette extraction).
	MaxColors int `json:"maxColors"`
	// Colors is an explicit palette in #rgb / #rrggbb form.
	Colors         []string   `json:"colors"`
	Dither         DitherMode `json:"dither"`
	DitherStrength float64    `json:"ditherStrength"`
	// Cleanup removes isolated opaque pixels left by anti-aliasing.
	Cleanup bool `json:"cleanup"`
	// Outline adds a 1px dark border around opaque pixels.
	Outline      bool   `json:"outline"`
	OutlineColor string `json:"outlineColor"`
	// CheckSeam compares opposite edges for tile wrapping.
	CheckSeam bool `json:"checkSeam"`
}

// SeamReport holds the mismatch fraction (0 = seamless) of opposite edges.
type SeamReport struct {
	LeftRight float64 `json:"leftRight"`
	TopBottom float64 `json:"topBottom"`
}

// Result is one processed image plus the palette and seam data used.
type Result struct {
	Image  *image.NRGBA
	Width  int
	Height int
	Colors []string
	Seam   SeamReport
	// Pad is the transparent-to-outline border added per side when the art
	// touched the canvas edge; pivots shift by the same amount.
	Pad int `json:"pad"`
	// Options is the normalised request, echoed so callers can persist it.
	Options Options
}

const maxTargetSize = 2048

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (opts Options) normalized() Options {
	opts.Width = clampInt(opts.Width, 0, maxTargetSize)
	opts.Height = clampInt(opts.Height, 0, maxTargetSize)
	if opts.AlphaThreshold <= 0 {
		opts.AlphaThreshold = 128
	}
	opts.AlphaThreshold = clampInt(opts.AlphaThreshold, 1, 255)
	if opts.MaxColors <= 0 {
		opts.MaxColors = 16
	}
	opts.MaxColors = clampInt(opts.MaxColors, 2, 64)
	switch opts.Dither {
	case DitherFloyd, DitherBayer, DitherNoise:
	default:
		opts.Dither = DitherNone
	}
	if opts.Dither == DitherNone {
		opts.DitherStrength = 0
	} else if opts.DitherStrength <= 0 {
		opts.DitherStrength = 1
	}
	opts.DitherStrength = math.Min(1, math.Max(0.05, opts.DitherStrength))
	// An empty OutlineColor means "darkest palette colour", which keeps the
	// result inside the palette. Plain text is parsed as an explicit colour.
	opts.OutlineColor = strings.TrimSpace(opts.OutlineColor)
	if len(opts.Colors) > 256 {
		opts.Colors = opts.Colors[:256]
	}
	return opts
}

// Process runs the full chain and never mutates the source image.
func Process(src image.Image, raw Options) Result {
	opts := raw.normalized()
	// Keep the original alpha through downsampling: coverage is averaged per
	// box and only quantisation binarises it. Binarising first would clip soft
	// AI edges before their coverage is counted.
	img := toNRGBA(src)
	if opts.Trim {
		img = trimTransparent(img, opts.AlphaThreshold)
	}
	img = resize(img, opts.Width, opts.Height)
	palette := paletteFor(img, opts)
	out := quantize(img, palette, opts)
	if opts.Cleanup {
		out = despeckle(out)
	}
	var outlineColor color.NRGBA
	pad := 0
	if opts.Outline {
		outlineColor = resolveOutlineColor(opts, palette)
		out, pad = addOutline(out, outlineColor)
	}
	colors := hexList(palette)
	if opts.Outline && len(palette) > 0 {
		key := hexOf(outlineColor)
		if !containsHex(colors, key) {
			colors = append(colors, key)
		}
	}
	result := Result{
		Image:   out,
		Width:   out.Bounds().Dx(),
		Height:  out.Bounds().Dy(),
		Colors:  colors,
		Pad:     pad,
		Options: opts,
	}
	if opts.CheckSeam {
		result.Seam = seamReport(out)
	}
	return result
}

func toNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// trimTransparent crops to the bounding box of pixels that will survive the
// alpha threshold. A fully transparent image is returned untouched so a size
// is still produced.
func trimTransparent(src *image.NRGBA, threshold int) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if int(src.NRGBAAt(x, y).A) < threshold {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, maxX-minX+1, maxY-minY+1))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			dst.SetNRGBA(x, y, src.NRGBAAt(minX+x, minY+y))
		}
	}
	return dst
}

// resize box-averages when shrinking (anti-aliased alpha collapses back to a
// hard edge at quantisation) and nearest-neighbour upsamples small sprites.
func resize(src *image.NRGBA, wantW, wantH int) *image.NRGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return src
	}
	if wantW <= 0 && wantH <= 0 {
		return src
	}
	if wantW <= 0 {
		wantW = int(math.Round(float64(sw) * float64(wantH) / float64(sh)))
	}
	if wantH <= 0 {
		wantH = int(math.Round(float64(sh) * float64(wantW) / float64(sw)))
	}
	wantW = clampInt(wantW, 1, maxTargetSize)
	wantH = clampInt(wantH, 1, maxTargetSize)
	if wantW == sw && wantH == sh {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, wantW, wantH))

	if wantW >= sw && wantH >= sh {
		for y := 0; y < wantH; y++ {
			sy := y * sh / wantH
			for x := 0; x < wantW; x++ {
				dst.SetNRGBA(x, y, src.NRGBAAt(x*sw/wantW, sy))
			}
		}
		return dst
	}

	clamp8 := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(math.Round(v))
	}
	for dy := 0; dy < wantH; dy++ {
		y0 := dy * sh / wantH
		y1 := (dy + 1) * sh / wantH
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for dx := 0; dx < wantW; dx++ {
			x0 := dx * sw / wantW
			x1 := (dx + 1) * sw / wantW
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sumR, sumG, sumB, sumA float64
			count := 0
			for sy := y0; sy < y1 && sy < sh; sy++ {
				for sx := x0; sx < x1 && sx < sw; sx++ {
					c := src.NRGBAAt(sx, sy)
					a := float64(c.A)
					sumR += float64(c.R) * a
					sumG += float64(c.G) * a
					sumB += float64(c.B) * a
					sumA += a
					count++
				}
			}
			if count == 0 || sumA == 0 {
				continue
			}
			dst.SetNRGBA(dx, dy, color.NRGBA{
				R: clamp8(sumR / sumA),
				G: clamp8(sumG / sumA),
				B: clamp8(sumB / sumA),
				A: clamp8(sumA / float64(count)),
			})
		}
	}
	return dst
}

// despeckle clears opaque pixels with no opaque 4-neighbour.
func despeckle(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(dst.Pix, src.Pix)
	opaque := func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		return src.NRGBAAt(x, y).A > 0
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !opaque(x, y) {
				continue
			}
			if opaque(x-1, y) || opaque(x+1, y) || opaque(x, y-1) || opaque(x, y+1) {
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{})
		}
	}
	return dst
}

// resolveOutlineColor prefers an explicit colour, then the darkest palette
// entry, so outlines stay inside the palette lock.
func resolveOutlineColor(opts Options, palette []color.NRGBA) color.NRGBA {
	if opts.OutlineColor != "" {
		return parseColor(opts.OutlineColor, defaultOutline)
	}
	if len(palette) > 0 {
		return darkestColor(palette)
	}
	return defaultOutline
}

var defaultOutline = color.NRGBA{R: 12, G: 16, B: 22, A: 255}

// addOutline paints transparent neighbours of opaque pixels. Pixel art that
// touches the canvas edge would lose its border, so the canvas grows by one
// pixel on every side before painting; art with a margin keeps its size.
func addOutline(src *image.NRGBA, col color.NRGBA) (*image.NRGBA, int) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w == 0 || h == 0 {
		return src, 0
	}
	pad := 0
	for x := 0; x < w && pad == 0; x++ {
		if src.NRGBAAt(x, 0).A > 0 || src.NRGBAAt(x, h-1).A > 0 {
			pad = 1
		}
	}
	for y := 0; y < h && pad == 0; y++ {
		if src.NRGBAAt(0, y).A > 0 || src.NRGBAAt(w-1, y).A > 0 {
			pad = 1
		}
	}
	if pad == 1 {
		// Always pad both axes so the art keeps a symmetric margin.
		dst := image.NewNRGBA(image.Rect(0, 0, w+2, h+2))
		draw.Draw(dst, image.Rect(1, 1, w+1, h+1), src, image.Point{}, draw.Src)
		paintOutline(dst, src, pad, col)
		return dst, pad
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(dst.Pix, src.Pix)
	paintOutline(dst, src, 0, col)
	return dst, 0
}

func paintOutline(dst, body *image.NRGBA, pad int, col color.NRGBA) {
	w, h := body.Bounds().Dx(), body.Bounds().Dy()
	dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
	opaque := func(x, y int) bool {
		if x < pad || y < pad || x >= pad+w || y >= pad+h {
			return false
		}
		return body.NRGBAAt(x-pad, y-pad).A > 0
	}
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			if opaque(x, y) {
				continue
			}
			if opaque(x-1, y) || opaque(x+1, y) || opaque(x, y-1) || opaque(x, y+1) {
				dst.SetNRGBA(x, y, col)
			}
		}
	}
}

// seamReport measures how well opposite edges match when the image tiles.
func seamReport(img *image.NRGBA) SeamReport {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w < 2 || h < 2 {
		return SeamReport{}
	}
	mismatch := func(a, b color.NRGBA) bool {
		if (a.A == 0) != (b.A == 0) {
			return true
		}
		if a.A == 0 {
			return false
		}
		dr := int(a.R) - int(b.R)
		dg := int(a.G) - int(b.G)
		db := int(a.B) - int(b.B)
		return dr*dr+dg*dg+db*db > 24*24*3
	}
	lr, tb := 0, 0
	for y := 0; y < h; y++ {
		if mismatch(img.NRGBAAt(0, y), img.NRGBAAt(w-1, y)) {
			lr++
		}
	}
	for x := 0; x < w; x++ {
		if mismatch(img.NRGBAAt(x, 0), img.NRGBAAt(x, h-1)) {
			tb++
		}
	}
	round := func(v float64) float64 { return math.Round(v*1000) / 1000 }
	return SeamReport{
		LeftRight: round(float64(lr) / float64(h)),
		TopBottom: round(float64(tb) / float64(w)),
	}
}

func parseColor(value string, fallback color.NRGBA) color.NRGBA {
	text := strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if len(text) == 3 {
		text = string([]byte{text[0], text[0], text[1], text[1], text[2], text[2]})
	}
	if len(text) != 6 {
		return fallback
	}
	n, err := strconv.ParseUint(text, 16, 32)
	if err != nil {
		return fallback
	}
	return color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
}
