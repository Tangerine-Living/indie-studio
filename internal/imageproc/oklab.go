package imageproc

import (
	"image/color"
	"math"
)

// oklab is a perceptually uniform colour space: nearest-palette matching in it
// avoids the hue shifts plain RGB distance produces.
type oklab struct {
	l float64
	a float64
	b float64
}

func srgbToLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func toOKLab(c color.NRGBA) oklab {
	r := srgbToLinear(float64(c.R) / 255)
	g := srgbToLinear(float64(c.G) / 255)
	b := srgbToLinear(float64(c.B) / 255)

	l := 0.4122214708*r + 0.5363325363*g + 0.0514459929*b
	m := 0.2119034982*r + 0.6806995451*g + 0.1073969566*b
	s := 0.0883024619*r + 0.2817188376*g + 0.6299787005*b

	lRoot := math.Cbrt(l)
	mRoot := math.Cbrt(m)
	sRoot := math.Cbrt(s)

	return oklab{
		l: 0.2104542553*lRoot + 0.7936177850*mRoot - 0.0040720468*sRoot,
		a: 1.9779984951*lRoot - 2.4285922050*mRoot + 0.4505937099*sRoot,
		b: 0.0259040371*lRoot + 0.7827717662*mRoot - 0.8086757660*sRoot,
	}
}

func (x oklab) distanceSquared(y oklab) float64 {
	dl := x.l - y.l
	da := x.a - y.a
	db := x.b - y.b
	// L* matters a little less to the eye than chroma hue; keep plain distance
	// for predictability, it is already perceptually scaled.
	return dl*dl + da*da + db*db
}

// nearestColor finds the closest palette entry in OKLab.
func nearestColor(target oklab, palette []color.NRGBA, labs []oklab) color.NRGBA {
	best := 0
	bestDistance := math.Inf(1)
	for i, lab := range labs {
		d := target.distanceSquared(lab)
		if d < bestDistance {
			bestDistance = d
			best = i
		}
	}
	return palette[best]
}
