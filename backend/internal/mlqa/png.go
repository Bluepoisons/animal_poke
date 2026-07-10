package mlqa

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// SyntheticPNG generates a deterministic tiny PNG for a fixture.
// Colors are seeded so fixtures differ without storing binary assets.
func SyntheticPNG(spec ImageSpec) ([]byte, error) {
	w, h := spec.Width, spec.Height
	if w <= 0 {
		w = 32
	}
	if h <= 0 {
		h = 32
	}
	// Cap to keep tests light.
	if w > 256 {
		w = 256
	}
	if h > 256 {
		h = 256
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(spec.Seed)
	if seed == 0 {
		seed = 1
	}
	// Background
	bg := color.RGBA{
		R: byte(30 + (seed*17)%200),
		G: byte(40 + (seed*29)%180),
		B: byte(50 + (seed*43)%160),
		A: 255,
	}
	for y := range h {
		for x := range w {
			img.Set(x, y, bg)
		}
	}
	// Foreground blob (stand-in for subject region)
	fg := color.RGBA{
		R: byte(200 - (seed*7)%150),
		G: byte(180 - (seed*11)%140),
		B: byte(160 - (seed*13)%120),
		A: 255,
	}
	x0 := w / 4
	y0 := h / 4
	x1 := (3 * w) / 4
	y1 := (3 * h) / 4
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			// checker noise from seed so compression/blur fixtures still unique
			if ((x + y + int(seed)) % 3) == 0 {
				img.Set(x, y, fg)
			} else {
				img.Set(x, y, color.RGBA{R: fg.R / 2, G: fg.G / 2, B: fg.B / 2, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
