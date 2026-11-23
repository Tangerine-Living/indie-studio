package generate

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"pixelbench/internal/apperr"
)

// decodeImageDataURL accepts a data: URL (or bare base64) and returns bytes.
func decodeImageDataURL(raw string) ([]byte, error) {
	data := strings.TrimSpace(raw)
	if data == "" {
		return nil, apperr.New("INVALID_INPAINT", "缺少原图或蒙版")
	}
	if strings.HasPrefix(data, "data:") {
		comma := strings.Index(data, ",")
		if comma < 0 {
			return nil, apperr.New("INVALID_INPAINT", "原图或蒙版的数据格式不正确")
		}
		data = data[comma+1:]
	}
	out, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(out) == 0 {
		return nil, apperr.New("INVALID_INPAINT", "无法解析原图或蒙版")
	}
	return out, nil
}

// maskStrength returns 0..255 per pixel: white/opaque means "regenerate here".
func maskStrength(maskPNG []byte) (*image.NRGBA, error) {
	src, _, err := image.Decode(bytes.NewReader(maskPNG))
	if err != nil {
		return nil, apperr.Wrap("INVALID_INPAINT", "蒙版不是有效的 PNG", err)
	}
	bounds := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	outBounds := out.Bounds()
	draw.Draw(out, outBounds, src, bounds.Min, draw.Src)
	for y := 0; y < outBounds.Dy(); y++ {
		for x := 0; x < outBounds.Dx(); x++ {
			c := out.NRGBAAt(x, y)
			peak := c.R
			if c.G > peak {
				peak = c.G
			}
			if c.B > peak {
				peak = c.B
			}
			// Colour carries the painted amount; alpha scales it so a
			// half-transparent white stroke reads as a partial edit.
			strength := uint8(uint16(peak) * uint16(c.A) / 255)
			out.SetNRGBA(x, y, color.NRGBA{R: strength, G: strength, B: strength, A: 255})
		}
	}
	return out, nil
}

// maskForOpenAI flips the painted mask into OpenAI's convention: transparent
// where the image should be edited, opaque elsewhere.
func maskForOpenAI(maskPNG []byte) ([]byte, error) {
	strength, err := maskStrength(maskPNG)
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(strength.Bounds())
	for y := 0; y < strength.Bounds().Dy(); y++ {
		for x := 0; x < strength.Bounds().Dx(); x++ {
			painted := strength.NRGBAAt(x, y).R
			out.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255 - painted})
		}
	}
	return encodeNRGBA(out)
}

// maskForComfy keeps white = regenerate, as ComfyUI mask loaders expect.
func maskForComfy(maskPNG []byte) ([]byte, error) {
	strength, err := maskStrength(maskPNG)
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(strength.Bounds())
	for y := 0; y < strength.Bounds().Dy(); y++ {
		for x := 0; x < strength.Bounds().Dx(); x++ {
			value := strength.NRGBAAt(x, y).R
			// Mirror the value into alpha too, so LoadImageMask works with
			// either the red channel or the alpha channel.
			out.SetNRGBA(x, y, color.NRGBA{R: value, G: value, B: value, A: value})
		}
	}
	return encodeNRGBA(out)
}

func encodeNRGBA(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, apperr.Wrap("INVALID_INPAINT", "无法编码蒙版", err)
	}
	return buf.Bytes(), nil
}
