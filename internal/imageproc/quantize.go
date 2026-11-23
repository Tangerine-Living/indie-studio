package imageproc

import (
	"image"
	"image/color"
	"math"
)

func clampChannel(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(math.Round(v))
}

// quantize snaps every opaque pixel to the palette, optionally dithering.
func quantize(img *image.NRGBA, palette []color.NRGBA, opts Options) *image.NRGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	alpha := opts.AlphaThreshold
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if len(palette) == 0 {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := img.NRGBAAt(x, y)
				if int(c.A) >= alpha {
					c.A = 255
					dst.SetNRGBA(x, y, c)
				}
			}
		}
		return dst
	}

	labs := make([]oklab, len(palette))
	for i, c := range palette {
		labs[i] = toOKLab(c)
	}

	switch opts.Dither {
	case DitherFloyd:
		floydSteinberg(img, dst, palette, labs, opts.DitherStrength, alpha)
	case DitherBayer:
		orderedDither(img, dst, palette, labs, opts.DitherStrength, alpha, bayerThreshold)
	case DitherNoise:
		orderedDither(img, dst, palette, labs, opts.DitherStrength, alpha, noiseThreshold)
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := img.NRGBAAt(x, y)
				if int(c.A) < alpha {
					continue
				}
				c.A = 255
				pick := nearestColor(toOKLab(c), palette, labs)
				dst.SetNRGBA(x, y, pick)
			}
		}
	}
	return dst
}

func floydSteinberg(
	img *image.NRGBA,
	dst *image.NRGBA,
	palette []color.NRGBA,
	labs []oklab,
	strength float64,
	alphaThreshold int,
) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	bufR := make([]float64, w*h)
	bufG := make([]float64, w*h)
	bufB := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			idx := y*w + x
			bufR[idx] = float64(c.R)
			bufG[idx] = float64(c.G)
			bufB[idx] = float64(c.B)
		}
	}
	spread := func(x, y int, errR, errG, errB, weight float64) {
		if x < 0 || y < 0 || x >= w || y >= h {
			return
		}
		if int(img.NRGBAAt(x, y).A) < alphaThreshold {
			return
		}
		idx := y*w + x
		bufR[idx] += errR * weight
		bufG[idx] += errG * weight
		bufB[idx] += errB * weight
	}
	for y := 0; y < h; y++ {
		reverse := y%2 == 1
		for step := 0; step < w; step++ {
			x := step
			dir := 1
			if reverse {
				x = w - 1 - step
				dir = -1
			}
			c := img.NRGBAAt(x, y)
			if int(c.A) < alphaThreshold {
				continue
			}
			idx := y*w + x
			adjusted := color.NRGBA{
				R: clampChannel(bufR[idx]),
				G: clampChannel(bufG[idx]),
				B: clampChannel(bufB[idx]),
				A: 255,
			}
			pick := nearestColor(toOKLab(adjusted), palette, labs)
			dst.SetNRGBA(x, y, pick)
			// Error is measured after clamping, so it never runs away.
			errR := (float64(adjusted.R) - float64(pick.R)) * strength
			errG := (float64(adjusted.G) - float64(pick.G)) * strength
			errB := (float64(adjusted.B) - float64(pick.B)) * strength
			spread(x+dir, y, errR, errG, errB, 7.0/16)
			spread(x-dir, y+1, errR, errG, errB, 3.0/16)
			spread(x, y+1, errR, errG, errB, 5.0/16)
			spread(x+dir, y+1, errR, errG, errB, 1.0/16)
		}
	}
}

// bayerMatrix is a 4x4 ordered-dither matrix in 0..15.
var bayerMatrix = [16]int{
	0, 8, 2, 10,
	12, 4, 14, 6,
	3, 11, 1, 9,
	15, 7, 13, 5,
}

func noiseThreshold(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h&0xffff) / 65535
}

// orderedDither applies a threshold map in sRGB space before palette lookup.
func orderedDither(
	img *image.NRGBA,
	dst *image.NRGBA,
	palette []color.NRGBA,
	labs []oklab,
	strength float64,
	alphaThreshold int,
	thresholdAt func(x, y int) float64,
) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	const amplitude = 48.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			if int(c.A) < alphaThreshold {
				continue
			}
			t := thresholdAt(x, y)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			offset := (t - 0.5) * amplitude * strength
			adjusted := color.NRGBA{
				R: clampChannel(float64(c.R) + offset),
				G: clampChannel(float64(c.G) + offset),
				B: clampChannel(float64(c.B) + offset),
				A: 255,
			}
			pick := nearestColor(toOKLab(adjusted), palette, labs)
			dst.SetNRGBA(x, y, pick)
		}
	}
}

func bayerThreshold(x, y int) float64 {
	index := (y%4)*4 + x%4
	return (float64(bayerMatrix[index]) + 0.5) / 16
}
