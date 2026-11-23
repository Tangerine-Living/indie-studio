package assets

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/imageproc"
	"pixelbench/internal/project"
)

// AISheetOptions describes how one AI image is cut into a cleaned,
// palette-locked sprite sheet. The AI is asked for a grid of frames; each cell
// is keyed, trimmed, scaled by one shared factor and bottom-centre aligned, so
// sprites do not jitter between frames.
type AISheetOptions struct {
	Name          string   `json:"name"`
	AssetType     string   `json:"assetType"`
	Columns       int      `json:"columns"`
	Rows          int      `json:"rows"`
	FrameSize     int      `json:"frameSize"`
	FPS           int      `json:"fps"`
	Tags          []string `json:"tags"`
	PaletteID     string   `json:"paletteId"`
	MaxColors     int      `json:"maxColors"`
	Outline       bool     `json:"outline"`
	KeyBackground bool     `json:"keyBackground"`
	Provider      string   `json:"provider"`
	Seed          int64    `json:"seed"`
}

const (
	maxAISheetFrames    = 64
	minAISheetFrameSize = 8
	maxAISheetFrameSize = 512
)

func clampAISheet(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (o AISheetOptions) normalized() (AISheetOptions, error) {
	o.Name = strings.TrimSpace(o.Name)
	o.AssetType = strings.ToLower(strings.TrimSpace(o.AssetType))
	if o.AssetType == "" {
		o.AssetType = project.AssetSprite
	}
	o.Columns = clampAISheet(o.Columns, 1, 16)
	o.Rows = clampAISheet(o.Rows, 1, 16)
	if o.Columns*o.Rows > maxAISheetFrames {
		return o, apperr.New("INVALID_SHEET", "一张精灵表最多 64 帧")
	}
	o.FrameSize = clampAISheet(o.FrameSize, minAISheetFrameSize, maxAISheetFrameSize)
	o.FPS = clampAISheet(o.FPS, 1, 60)
	o.MaxColors = clampAISheet(o.MaxColors, 2, 64)
	o.Tags = normalizeTags(o.Tags)
	return o, nil
}

// CreateAISheet slices src into opts.Columns×opts.Rows cells, cleans every
// frame with the pixel-perfect chain and stores the result as an animation
// asset. It never writes anything when composition fails.
func CreateAISheet(store *project.Store, src []byte, raw AISheetOptions) (project.Asset, error) {
	opts, err := raw.normalized()
	if err != nil {
		return project.Asset{}, err
	}
	if len(src) == 0 {
		return project.Asset{}, apperr.New("INVALID_SHEET", "生成结果为空")
	}
	if len(src) > MaxVersionBytes {
		return project.Asset{}, apperr.New("INVALID_SHEET", "生成结果超过 32 MB")
	}
	decoded, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return project.Asset{}, apperr.New("INVALID_SHEET", "生成结果不是有效的 PNG")
	}
	img := toNRGBA(decoded)
	if opts.KeyBackground {
		img = keyFlatBackground(img, 24)
	}
	if img.Bounds().Dx() < opts.Columns || img.Bounds().Dy() < opts.Rows {
		return project.Asset{}, apperr.New("INVALID_SHEET", "生成图太小，无法切成指定行列")
	}

	imgW, imgH := img.Bounds().Dx(), img.Bounds().Dy()
	cellW, cellH := imgW/opts.Columns, imgH/opts.Rows
	if cellW < 4 || cellH < 4 {
		return project.Asset{}, apperr.New("INVALID_SHEET", "每个格子至少需要 4×4 像素")
	}
	count := opts.Columns * opts.Rows
	trimmed := make([]*image.NRGBA, count)
	maxW, maxH := 1, 1
	for index := 0; index < count; index++ {
		col, row := index%opts.Columns, index/opts.Columns
		// Distribute any leftover pixels instead of dropping a right/bottom
		// strip when the image size is not divisible by the grid.
		x0, x1 := col*imgW/opts.Columns, (col+1)*imgW/opts.Columns
		y0, y1 := row*imgH/opts.Rows, (row+1)*imgH/opts.Rows
		cell := image.NewNRGBA(image.Rect(0, 0, x1-x0, y1-y0))
		draw.Draw(cell, cell.Bounds(), img, image.Pt(x0, y0), draw.Src)
		trimmed[index] = cropOpaque(cell)
		if w, h := trimmed[index].Bounds().Dx(), trimmed[index].Bounds().Dy(); w > 0 && h > 0 {
			if w > maxW {
				maxW = w
			}
			if h > maxH {
				maxH = h
			}
		}
	}

	// One scale for every frame keeps relative sizes stable; the largest frame
	// defines the target box.
	scale := float64(opts.FrameSize) / float64(maxW)
	if float64(opts.FrameSize)/float64(maxH) < scale {
		scale = float64(opts.FrameSize) / float64(maxH)
	}
	colors := paletteColors(store, opts.PaletteID)
	if len(colors) == 0 {
		// Every frame must share one palette, otherwise auto-extraction gives
		// each frame its own colours and the animation flickers.
		colors = imageproc.ExtractPaletteFor(trimmed, opts.MaxColors)
	}
	frames := make([]*image.NRGBA, count)
	for index, cell := range trimmed {
		if cell.Bounds().Dx() == 0 || cell.Bounds().Dy() == 0 {
			frames[index] = image.NewNRGBA(image.Rect(0, 0, 0, 0))
			continue
		}
		w := clampAISheet(int(math.Round(float64(cell.Bounds().Dx())*scale)), 1, opts.FrameSize)
		h := clampAISheet(int(math.Round(float64(cell.Bounds().Dy())*scale)), 1, opts.FrameSize)
		processed := imageproc.Process(cell, imageproc.Options{
			Width:          w,
			Height:         h,
			MaxColors:      opts.MaxColors,
			Colors:         colors,
			AlphaThreshold: 128,
			Cleanup:        true,
			Outline:        opts.Outline,
			// Empty means the darkest shared palette colour.
			OutlineColor: "",
		})
		frames[index] = processed.Image
	}

	// Frames are trimmed to content, so an outline can grow them by one pixel per
	// side; the canvas is sized to hold that border.
	canvasW := opts.FrameSize + 2
	canvasH := opts.FrameSize + 2
	sheet := image.NewNRGBA(image.Rect(0, 0, canvasW*opts.Columns, canvasH*opts.Rows))
	for index, frame := range frames {
		if frame.Bounds().Dx() == 0 || frame.Bounds().Dy() == 0 {
			continue
		}
		col, row := index%opts.Columns, index/opts.Columns
		fw, fh := frame.Bounds().Dx(), frame.Bounds().Dy()
		x := col*canvasW + (canvasW-fw)/2
		y := row*canvasH + (canvasH - fh) - 1
		if y < row*canvasH {
			y = row * canvasH
		}
		draw.Draw(sheet, image.Rect(x, y, x+fw, y+fh), frame, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		return project.Asset{}, apperr.Wrap("IO", "无法编码精灵表", err)
	}

	now := time.Now().UTC()
	label := opts.Name
	if label == "" {
		label = "AI 精灵表"
	}
	tags := append([]string{"generated", "AI"}, opts.Tags...)
	skeleton := project.Asset{
		SchemaVersion: project.SchemaVersion,
		Name:          label,
		Type:          opts.AssetType,
		Tags:          tags,
		Status:        project.AssetDraft,
		Dimensions:    project.Dimensions{W: canvasW, H: canvasH},
		Pivot:         project.Point{X: canvasW / 2, Y: canvasH - 1},
		PaletteID:     opts.PaletteID,
		License:       "generated",
		Source: project.AssetSource{
			Kind:     "generated",
			Provider: opts.Provider,
			Workflow: "ai-sheet",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	saved, err := store.SaveAsset(skeleton)
	if err != nil {
		return project.Asset{}, err
	}
	info := AnimationInfo{
		FrameWidth:  canvasW,
		FrameHeight: canvasH,
		FrameCount:  count,
		Columns:     opts.Columns,
		Rows:        opts.Rows,
		FPS:         opts.FPS,
		Loop:        true,
		Tags:        opts.Tags,
	}
	asset, err := SaveSheet(store, saved.ID, buf.Bytes(), info)
	if err != nil {
		// Keep the project tidy: the skeleton has no usable frames.
		_ = store.DeleteAsset(saved.ID)
		return project.Asset{}, err
	}
	return asset, nil
}

func toNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func cropOpaque(src *image.NRGBA) *image.NRGBA {
	b := src.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			// Match the pipeline's alpha threshold so a semi-transparent halo
			// never inflates the frame box or the shared scale.
			if src.NRGBAAt(x, y).A < 128 {
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
	if minX > maxX || minY > maxY {
		return image.NewNRGBA(image.Rect(0, 0, 0, 0))
	}
	out := image.NewNRGBA(image.Rect(0, 0, maxX-minX+1, maxY-minY+1))
	draw.Draw(out, out.Bounds(), src, image.Pt(minX, minY), draw.Src)
	return out
}

// keyFlatBackground removes a flat backdrop from the border inwards. AI
// outputs honour transparent-background prompts most of the time; when they do
// not, a uniform or near-uniform border is keyed out.
func keyFlatBackground(src *image.NRGBA, tolerance int) *image.NRGBA {
	b := src.Bounds()
	if b.Dx() < 2 || b.Dy() < 2 {
		return src
	}
	corners := []color.NRGBA{
		src.NRGBAAt(b.Min.X, b.Min.Y),
		src.NRGBAAt(b.Max.X-1, b.Min.Y),
		src.NRGBAAt(b.Min.X, b.Max.Y-1),
		src.NRGBAAt(b.Max.X-1, b.Max.Y-1),
	}
	base := corners[0]
	if base.A == 0 {
		return src
	}
	for _, c := range corners[1:] {
		if c.A == 0 || colorDistance(c, base) > tolerance {
			return src
		}
	}
	match := func(c color.NRGBA) bool {
		return c.A > 0 && colorDistance(c, base) <= tolerance
	}
	visited := make([]bool, b.Dx()*b.Dy())
	queue := make([][2]int, 0, b.Dx()*2+b.Dy()*2)
	push := func(x, y int) {
		if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
			return
		}
		index := (y-b.Min.Y)*b.Dx() + (x - b.Min.X)
		if visited[index] {
			return
		}
		if !match(src.NRGBAAt(x, y)) {
			return
		}
		visited[index] = true
		queue = append(queue, [2]int{x, y})
	}
	for x := b.Min.X; x < b.Max.X; x++ {
		push(x, b.Min.Y)
		push(x, b.Max.Y-1)
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		push(b.Min.X, y)
		push(b.Max.X-1, y)
	}
	for head := 0; head < len(queue); head++ {
		point := queue[head]
		src.SetNRGBA(point[0], point[1], color.NRGBA{})
		push(point[0]-1, point[1])
		push(point[0]+1, point[1])
		push(point[0], point[1]-1)
		push(point[0], point[1]+1)
	}
	return src
}

func colorDistance(a, b color.NRGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if dr > dg {
		if dr > db {
			return dr
		}
		return db
	}
	if dg > db {
		return dg
	}
	return db
}

func paletteColors(store *project.Store, id string) []string {
	if id == "" {
		return nil
	}
	palettes, err := store.ListPalettes()
	if err != nil {
		return nil
	}
	for _, palette := range palettes {
		if palette.ID == id {
			return palette.Colors
		}
	}
	return nil
}