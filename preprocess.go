package goddddocr

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
)

const ocrTargetHeight = 64

func preprocessOCRImage(img image.Image, pngFix bool) ([]float32, int, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, 0, fmt.Errorf("empty image")
	}

	targetWidth := int(float64(width) * (float64(ocrTargetHeight) / float64(height)))
	if targetWidth <= 0 {
		targetWidth = 1
	}

	src := normalizeToNRGBA(img, pngFix)
	resized := imaging.Resize(src, targetWidth, ocrTargetHeight, imaging.Lanczos)

	data := make([]float32, targetWidth*ocrTargetHeight)
	for y := 0; y < ocrTargetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			c := color.NRGBAModel.Convert(resized.At(x, y)).(color.NRGBA)
			gray := grayscalePILLike(c.R, c.G, c.B)
			data[y*targetWidth+x] = float32(gray) / 255.0
		}
	}

	return data, targetWidth, nil
}

func normalizeToNRGBA(img image.Image, pngFix bool) *image.NRGBA {
	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			if pngFix && c.A < 255 {
				alpha := uint32(c.A)
				c.R = uint8((uint32(c.R)*alpha + 255*(255-alpha) + 127) / 255)
				c.G = uint8((uint32(c.G)*alpha + 255*(255-alpha) + 127) / 255)
				c.B = uint8((uint32(c.B)*alpha + 255*(255-alpha) + 127) / 255)
				c.A = 255
			}
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst
}

func grayscalePILLike(r, g, b uint8) uint8 {
	v := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	return uint8(math.Round(v))
}
