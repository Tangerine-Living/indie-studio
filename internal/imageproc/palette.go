package imageproc

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
)

// paletteFor honours an explicit palette and otherwise extracts one with a
// weighted median cut.
func paletteFor(img *image.NRGBA, opts Options) []color.NRGBA {
	if explicit := parsePalette(opts.Colors); len(explicit) > 0 {
		return explicit
	}
	return extractPaletteFor([]*image.NRGBA{img}, opts.MaxColors, opts.AlphaThreshold)
}

func parsePalette(values []string) []color.NRGBA {
	seen := map[uint32]bool{}
	out := make([]color.NRGBA, 0, len(values))
	for _, raw := range values {
		c := parseColor(raw, color.NRGBA{A: 0})
		if c.A == 0 {
			continue
		}
		key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

func hexOf(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func hexList(palette []color.NRGBA) []string {
	out := make([]string, 0, len(palette))
	for _, c := range palette {
		out = append(out, hexOf(c))
	}
	return out
}

func containsHex(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// darkestColor picks the palette entry with the lowest perceived lightness;
// it is the natural outline colour for palette-locked pixel art.
func darkestColor(palette []color.NRGBA) color.NRGBA {
	best := palette[0]
	bestL := toOKLab(best).l
	for _, c := range palette[1:] {
		if l := toOKLab(c).l; l < bestL {
			bestL = l
			best = c
		}
	}
	return best
}

type colorCount struct {
	c   color.NRGBA
	lab oklab
	n   int
}

type paletteBox struct {
	entries []colorCount
	weight  int
}

// ExtractPaletteFor builds one palette shared by several images. Sprite sheet
// frames must quantise against the same colours, otherwise every frame drifts
// and the animation flickers. Transparent pixels are skipped, so keyed
// backgrounds never take palette slots.
func ExtractPaletteFor(images []*image.NRGBA, max int) []string {
	return hexList(extractPaletteFor(images, max, 128))
}

func extractPaletteFor(images []*image.NRGBA, max, alphaThreshold int) []color.NRGBA {
	counts := map[uint32]*colorCount{}
	for _, img := range images {
		if img == nil {
			continue
		}
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				c := img.NRGBAAt(x, y)
				if int(c.A) < alphaThreshold {
					continue
				}
				key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
				if entry, ok := counts[key]; ok {
					entry.n++
					continue
				}
				counts[key] = &colorCount{
					c:   color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255},
					lab: toOKLab(c),
					n:   1,
				}
			}
		}
	}
	return buildPalette(counts, max)
}

func buildPalette(counts map[uint32]*colorCount, max int) []color.NRGBA {
	if max < 1 {
		max = 1
	}
	if max > 64 {
		max = 64
	}
	if len(counts) == 0 {
		return nil
	}

	all := make([]colorCount, 0, len(counts))
	for _, entry := range counts {
		all = append(all, *entry)
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i].c, all[j].c
		if a.R != b.R {
			return a.R < b.R
		}
		if a.G != b.G {
			return a.G < b.G
		}
		return a.B < b.B
	})

	boxes := []paletteBox{{entries: all, weight: totalWeight(all)}}
	if len(all) <= max {
		return averages(all)
	}
	for len(boxes) < max {
		index := widestBox(boxes)
		if index < 0 {
			break
		}
		left, right := splitBox(boxes[index].entries)
		if len(left) == 0 || len(right) == 0 {
			// Mark this box as unsplittable and try the next one.
			boxes[index].weight = -1
			continue
		}
		boxes[index] = paletteBox{entries: left, weight: totalWeight(left)}
		boxes = append(boxes, paletteBox{entries: right, weight: totalWeight(right)})
	}

	out := make([]color.NRGBA, 0, len(boxes))
	for _, box := range boxes {
		if len(box.entries) == 0 {
			continue
		}
		out = append(out, averageOf(box.entries))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].R != out[j].R {
			return out[i].R < out[j].R
		}
		if out[i].G != out[j].G {
			return out[i].G < out[j].G
		}
		return out[i].B < out[j].B
	})
	return out
}

func totalWeight(entries []colorCount) int {
	total := 0
	for _, entry := range entries {
		total += entry.n
	}
	return total
}

// channelRange measures the widest OKLab axis in a box. Splitting in OKLab
// keeps palette slots where the eye sees difference, not where sRGB bits do.
func channelRange(entries []colorCount) (channel int, span float64) {
	if len(entries) == 0 {
		return 0, 0
	}
	minL, maxL := entries[0].lab.l, entries[0].lab.l
	minA, maxA := entries[0].lab.a, entries[0].lab.a
	minB, maxB := entries[0].lab.b, entries[0].lab.b
	for _, entry := range entries[1:] {
		lab := entry.lab
		minL, maxL = min(minL, lab.l), max(maxL, lab.l)
		minA, maxA = min(minA, lab.a), max(maxA, lab.a)
		minB, maxB = min(minB, lab.b), max(maxB, lab.b)
	}
	spanL, spanA, spanB := maxL-minL, maxA-minA, maxB-minB
	switch {
	case spanL >= spanA && spanL >= spanB:
		return 0, spanL
	case spanA >= spanB:
		return 1, spanA
	default:
		return 2, spanB
	}
}

func widestBox(boxes []paletteBox) int {
	best := -1
	bestScore := 0.0
	for i, box := range boxes {
		if box.weight <= 0 || len(box.entries) < 2 {
			continue
		}
		_, span := channelRange(box.entries)
		score := span * float64(box.weight)
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	return best
}

func splitBox(entries []colorCount) (left, right []colorCount) {
	channel, span := channelRange(entries)
	if span == 0 {
		return nil, nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		switch channel {
		case 0:
			return entries[i].lab.l < entries[j].lab.l
		case 1:
			return entries[i].lab.a < entries[j].lab.a
		default:
			return entries[i].lab.b < entries[j].lab.b
		}
	})
	total := totalWeight(entries)
	half := total / 2
	running := 0
	cut := 1
	for i, entry := range entries {
		running += entry.n
		if running >= half && i > 0 && i < len(entries)-1 {
			cut = i + 1
			break
		}
	}
	return append([]colorCount{}, entries[:cut]...), append([]colorCount{}, entries[cut:]...)
}

func averageOf(entries []colorCount) color.NRGBA {
	var sumR, sumG, sumB, sumN float64
	for _, entry := range entries {
		sumR += float64(entry.c.R) * float64(entry.n)
		sumG += float64(entry.c.G) * float64(entry.n)
		sumB += float64(entry.c.B) * float64(entry.n)
		sumN += float64(entry.n)
	}
	if sumN == 0 {
		return color.NRGBA{}
	}
	round := func(v float64) uint8 { return uint8(math.Round(v)) }
	return color.NRGBA{
		R: round(sumR / sumN),
		G: round(sumG / sumN),
		B: round(sumB / sumN),
		A: 255,
	}
}

func averages(entries []colorCount) []color.NRGBA {
	out := make([]color.NRGBA, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.c)
	}
	return out
}
