package goddddocr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
)

const ocrTargetHeight = 64

type PreprocessOptions struct {
	PNGFix      bool
	ColorFilter *ColorFilterOptions
}

type PreprocessResult struct {
	Width  int       `json:"width"`
	Height int       `json:"height"`
	Data   []float32 `json:"data"`
}

func PreprocessOCRBytes(data []byte, options *PreprocessOptions) (*PreprocessResult, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return PreprocessOCRImage(img, options)
}

func PreprocessOCRImage(img image.Image, options *PreprocessOptions) (*PreprocessResult, error) {
	pngFix := false
	var colorFilter *ColorFilterOptions
	if options != nil {
		pngFix = options.PNGFix
		colorFilter = options.ColorFilter
	}

	data, width, err := preprocessOCRImage(img, pngFix, colorFilter)
	if err != nil {
		return nil, err
	}
	return &PreprocessResult{
		Width:  width,
		Height: ocrTargetHeight,
		Data:   data,
	}, nil
}

func (r *PreprocessResult) GrayImage() (*image.Gray, error) {
	if r == nil || r.Width <= 0 || r.Height <= 0 {
		return nil, fmt.Errorf("invalid preprocess result dimensions")
	}
	if len(r.Data) != r.Width*r.Height {
		return nil, fmt.Errorf("preprocess data length %d does not match dimensions %dx%d", len(r.Data), r.Width, r.Height)
	}
	img := image.NewGray(image.Rect(0, 0, r.Width, r.Height))
	for idx, value := range r.Data {
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
		img.Pix[idx] = uint8(math.Round(float64(value) * 255.0))
	}
	return img, nil
}

func preprocessOCRImage(img image.Image, pngFix bool, colorFilter *ColorFilterOptions) ([]float32, int, error) {
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
	if colorFilter != nil {
		filtered, err := applyColorFilter(src, colorFilter)
		if err != nil {
			return nil, 0, fmt.Errorf("apply color filter: %w", err)
		}
		src = filtered
	}
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
