// Package generators draws procedural pixel art. Everything here is pure Go
// image code so results are deterministic for a given seed and can be tested
// with golden expectations.
package generators

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"strconv"
	"strings"
)

// BuildingOptions are the inputs of the top-down building generator.
type BuildingOptions struct {
	// Seed pins every random decision (window count, weathering, colour jitter).
	Seed      int64   `json:"seed"`
	Width     int     `json:"width"` // footprint in tiles
	Depth     int     `json:"depth"` // footprint in tiles
	TileSize  int     `json:"tileSize"`
	Floors    int     `json:"floors"`
	WallColor string  `json:"wallColor"`
	RoofColor string  `json:"roofColor"`
	TrimColor string  `json:"trimColor"`
	DoorColor string  `json:"doorColor"`
	Windows   int     `json:"windows"` // per floor; 0 = automatic
	Sign      bool    `json:"sign"`
	Weathered float64 `json:"weathered"` // 0..1
	Shadow    bool    `json:"shadow"`
	Outline   bool    `json:"outline"`
	Variants  int     `json:"variants"`
}

// Point is a pixel coordinate.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Rect is a pixel rectangle.
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Building is one generated variant. PNG holds the composite; BasePNG and
// RoofPNG are the layered halves used by roof-fade previews.
type Building struct {
	Seed      int64  `json:"seed"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Pivot     Point  `json:"pivot"`
	Collision Rect   `json:"collision"`
	PNG       []byte `json:"-"`
	BasePNG   []byte `json:"-"`
	RoofPNG   []byte `json:"-"`
}

const (
	maxFootprint = 16
	maxVariants  = 8
	defaultTile  = 32
)

func (opts BuildingOptions) normalized() BuildingOptions {
	clamp := func(value, min, max int) int {
		if value < min {
			return min
		}
		if value > max {
			return max
		}
		return value
	}
	opts.Width = clamp(opts.Width, 1, maxFootprint)
	opts.Depth = clamp(opts.Depth, 1, maxFootprint)
	opts.Floors = clamp(opts.Floors, 1, 4)
	opts.Variants = clamp(opts.Variants, 1, maxVariants)
	opts.Windows = clamp(opts.Windows, 0, 12)
	switch opts.TileSize {
	case 16, 24, 32, 48:
	default:
		opts.TileSize = defaultTile
	}
	if opts.Weathered < 0 {
		opts.Weathered = 0
	}
	if opts.Weathered > 1 {
		opts.Weathered = 1
	}
	return opts
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

func shade(c color.NRGBA, delta int) color.NRGBA {
	clamp := func(v int) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	return color.NRGBA{R: clamp(int(c.R) + delta), G: clamp(int(c.G) + delta), B: clamp(int(c.B) + delta), A: c.A}
}

func mix(a, b color.NRGBA, amount float64) color.NRGBA {
	if amount < 0 {
		amount = 0
	}
	if amount > 1 {
		amount = 1
	}
	blend := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-amount) + float64(y)*amount)
	}
	return color.NRGBA{R: blend(a.R, b.R), G: blend(a.G, b.G), B: blend(a.B, b.B), A: 255}
}

type canvas struct {
	img *image.NRGBA
}

func newCanvas(width, height int) *canvas {
	return &canvas{img: image.NewNRGBA(image.Rect(0, 0, width, height))}
}

func (c *canvas) fill(r Rect, col color.NRGBA) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			c.img.SetNRGBA(x, y, col)
		}
	}
}

func (c *canvas) set(x, y int, col color.NRGBA) {
	c.img.SetNRGBA(x, y, col)
}

func (c *canvas) frame(r Rect, col color.NRGBA) {
	for x := r.X; x < r.X+r.W; x++ {
		c.set(x, r.Y, col)
		c.set(x, r.Y+r.H-1, col)
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		c.set(r.X, y, col)
		c.set(r.X+r.W-1, y, col)
	}
}

func (c *canvas) encode() ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, c.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawRoof(c *canvas, r Rect, col color.NRGBA, rng *rand.Rand, weathered float64) {
	c.fill(r, col)
	eave := shade(col, -26)
	c.frame(r, eave)
	if r.H > 6 {
		// Shingle courses and a lighter ridge line halfway down.
		for y := r.Y + 4; y < r.Y+r.H-2; y += 4 {
			for x := r.X + 2; x < r.X+r.W-2; x++ {
				if (x+y)%7 == 0 {
					continue
				}
				c.set(x, y, shade(col, -14))
			}
		}
		ridgeY := r.Y + r.H/2
		c.fill(Rect{X: r.X + 1, Y: ridgeY - 1, W: r.W - 2, H: 2}, shade(col, 18))
	}
	// Eave shadow on the top edge keeps the roof reading as a slab.
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		c.set(x, r.Y+r.H-2, shade(col, -20))
	}
	if weathered > 0 {
		c.weather(r, rng, weathered*0.7)
	}
}

func (c *canvas) weather(r Rect, rng *rand.Rand, strength float64) {
	probability := strength * 0.06
	for y := r.Y + 1; y < r.Y+r.H-1; y++ {
		for x := r.X + 1; x < r.X+r.W-1; x++ {
			if rng.Float64() >= probability {
				continue
			}
			current := c.img.NRGBAAt(x, y)
			if current.A == 0 {
				continue
			}
			delta := 10
			if rng.Intn(2) == 0 {
				delta = -12
			}
			c.set(x, y, shade(current, delta))
		}
	}
}

func drawWall(c *canvas, r Rect, opts BuildingOptions, wall, trim, door color.NRGBA, rng *rand.Rand) {
	c.fill(r, wall)
	// Siding lines.
	for y := r.Y + 4; y < r.Y+r.H-2; y += 4 {
		for x := r.X + 2; x < r.X+r.W-2; x++ {
			c.set(x, y, shade(wall, -12))
		}
	}
	// Light from the top-left: right edge catches less light.
	c.fill(Rect{X: r.X, Y: r.Y, W: 2, H: r.H}, shade(wall, 10))
	c.fill(Rect{X: r.X + r.W - 2, Y: r.Y, W: 2, H: r.H}, shade(wall, -22))

	floorHeight := r.H / opts.Floors
	windowCount := opts.Windows
	if windowCount == 0 {
		windowCount = max(1, opts.Width-1)
	}
	windowW := max(4, opts.TileSize/2)
	windowH := max(4, min(opts.TileSize/2, floorHeight/2))
	inset := max(3, windowW/2)
	span := r.W - inset*2
	step := 0
	if windowCount > 1 {
		step = (span - windowW) / (windowCount - 1)
	}

	doorW := max(6, opts.TileSize*2/3)
	doorH := min(r.H, opts.TileSize)
	doorX := r.X + (r.W-doorW)/2
	doorY := r.Y + r.H - doorH

	for floor := 0; floor < opts.Floors; floor++ {
		centerY := r.Y + floor*floorHeight + floorHeight/2
		top := centerY - windowH/2
		for index := 0; index < windowCount; index++ {
			x := r.X + inset
			if windowCount > 1 {
				x += index * step
			}
			if x+windowW > r.X+r.W-inset {
				x = r.X + r.W - inset - windowW
			}
			// Keep ground-floor windows clear of the doorway.
			if floor == opts.Floors-1 && x < doorX+doorW+2 && x+windowW > doorX-2 {
				continue
			}
			glass := mix(color.NRGBA{R: 78, G: 140, B: 168, A: 255}, color.NRGBA{R: 240, G: 214, B: 150, A: 255}, rng.Float64()*0.35)
			frame := Rect{X: x, Y: top, W: windowW, H: windowH}
			c.fill(frame, trim)
			c.fill(Rect{X: x + 1, Y: top + 1, W: windowW - 2, H: windowH - 2}, glass)
			if windowW > 4 {
				c.fill(Rect{X: x + windowW/2, Y: top + 1, W: 1, H: windowH - 2}, trim)
			}
			if windowH > 4 {
				c.fill(Rect{X: x + 1, Y: top + windowH/2, W: windowW - 2, H: 1}, trim)
			}
			c.set(x+1, top+1, shade(glass, 40))
		}
	}

	doorRect := Rect{X: doorX, Y: doorY, W: doorW, H: doorH}
	c.fill(doorRect, trim)
	c.fill(Rect{X: doorX + 1, Y: doorY + 1, W: doorW - 2, H: doorH - 1}, door)
	c.fill(Rect{X: doorX + doorW/2, Y: doorY + doorH/2, W: 1, H: max(2, doorH/2-2)}, shade(door, -24))
	c.set(doorX+doorW-4, doorY+doorH/2, color.NRGBA{R: 242, G: 216, B: 120, A: 255})

	if opts.Sign {
		signW := doorW + interval(opts.TileSize)
		signH := max(5, opts.TileSize/2)
		signX := doorX + (doorW-signW)/2
		signY := doorY - signH - 3
		if signY > r.Y+2 {
			board := shade(trim, -18)
			c.fill(Rect{X: signX, Y: signY, W: signW, H: signH}, board)
			c.frame(Rect{X: signX, Y: signY, W: signW, H: signH}, shade(trim, -40))
			for x := signX + 3; x < signX+signW-3; x += 4 {
				c.fill(Rect{X: x, Y: signY + signH/2, W: 2, H: 1}, shade(board, 60))
			}
		}
	}

	if opts.Weathered > 0 {
		c.weather(r, rng, opts.Weathered)
	}
}

func interval(tile int) int {
	return max(6, tile/2)
}

// drawShadow paints an offset block shadow below-right of the footprint
// (light comes from the top-left, per the project style guide).
func drawShadow(c *canvas, footprint Rect, offset int) {
	dark := color.NRGBA{R: 8, G: 12, B: 18, A: 60}
	c.fill(Rect{
		X: footprint.X + offset,
		Y: footprint.Y + offset,
		W: footprint.W,
		H: footprint.H,
	}, dark)
}

func outline(c *canvas) {
	bounds := c.img.Bounds()
	opaque := func(x, y int) bool {
		if x < 0 || y < 0 || x >= bounds.Dx() || y >= bounds.Dy() {
			return false
		}
		return c.img.NRGBAAt(x, y).A > 0
	}
	edge := color.NRGBA{R: 12, G: 16, B: 22, A: 255}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			if opaque(x, y) {
				continue
			}
			if opaque(x-1, y) || opaque(x+1, y) || opaque(x, y-1) || opaque(x, y+1) {
				c.set(x, y, edge)
			}
		}
	}
}

// GenerateBuildings draws one top-down 3/4 building per variant.
func GenerateBuildings(raw BuildingOptions) ([]Building, error) {
	opts := raw.normalized()
	wall := parseColor(opts.WallColor, color.NRGBA{R: 200, G: 168, B: 124, A: 255})
	roof := parseColor(opts.RoofColor, color.NRGBA{R: 176, G: 78, B: 62, A: 255})
	trim := parseColor(opts.TrimColor, color.NRGBA{R: 120, G: 84, B: 58, A: 255})
	door := parseColor(opts.DoorColor, color.NRGBA{R: 92, G: 58, B: 36, A: 255})

	tile := opts.TileSize
	bodyW := opts.Width * tile
	bodyD := opts.Depth * tile
	wallH := max(tile, opts.Floors*tile*3/4)
	pad := 6
	eave := max(2, tile/6)
	shadowOffset := 0
	if opts.Shadow {
		shadowOffset = 4
	}

	width := pad*2 + bodyW
	height := pad*2 + bodyD + wallH + shadowOffset
	roofRect := Rect{X: pad - eave, Y: pad - eave, W: bodyW + eave*2, H: bodyD + eave*2}
	wallRect := Rect{X: pad, Y: pad + bodyD - eave, W: bodyW, H: wallH + eave}
	footprint := Rect{X: pad, Y: pad, W: bodyW, H: bodyD}

	results := make([]Building, 0, opts.Variants)
	for index := 0; index < opts.Variants; index++ {
		seed := opts.Seed + int64(index)*7919
		rng := rand.New(rand.NewSource(seed))
		variant := opts
		if opts.Windows == 0 && opts.Width > 2 && rng.Intn(3) == 0 {
			variant.Windows = max(1, opts.Width-2)
		}

		base := newCanvas(width, height)
		if opts.Shadow {
			drawShadow(base, footprint, shadowOffset)
		}
		drawWall(base, wallRect, variant, wall, trim, door, rng)

		roofLayer := newCanvas(width, height)
		drawRoof(roofLayer, roofRect, roof, rng, opts.Weathered)

		composite := newCanvas(width, height)
		if opts.Shadow {
			drawShadow(composite, footprint, shadowOffset)
		}
		draw.Draw(composite.img, composite.img.Bounds(), roofLayer.img, image.Point{}, draw.Over)
		draw.Draw(composite.img, composite.img.Bounds(), base.img, image.Point{}, draw.Over)
		if opts.Outline {
			outline(composite)
		}

		compositePNG, err := composite.encode()
		if err != nil {
			return nil, fmt.Errorf("encode building: %w", err)
		}
		basePNG, err := base.encode()
		if err != nil {
			return nil, fmt.Errorf("encode building base: %w", err)
		}
		roofPNG, err := roofLayer.encode()
		if err != nil {
			return nil, fmt.Errorf("encode building roof: %w", err)
		}
		results = append(results, Building{
			Seed:      seed,
			Width:     width,
			Height:    height,
			Pivot:     Point{X: pad + bodyW/2, Y: pad + bodyD + wallH},
			Collision: footprint,
			PNG:       compositePNG,
			BasePNG:   basePNG,
			RoofPNG:   roofPNG,
		})
	}
	return results, nil
}
