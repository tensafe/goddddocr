package goddddocr

import (
	"image"
	"image/color"
	"testing"
)

func TestPreprocessOCRImageResult(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(40 * x), G: uint8(50 * y), B: 100, A: 255})
		}
	}

	result, err := PreprocessOCRImage(img, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Width != 128 || result.Height != ocrTargetHeight {
		t.Fatalf("dimensions = %dx%d, want 128x%d", result.Width, result.Height, ocrTargetHeight)
	}
	if len(result.Data) != result.Width*result.Height {
		t.Fatalf("data length = %d, want %d", len(result.Data), result.Width*result.Height)
	}
	gray, err := result.GrayImage()
	if err != nil {
		t.Fatal(err)
	}
	if gray.Bounds().Dx() != result.Width || gray.Bounds().Dy() != result.Height {
		t.Fatalf("gray bounds = %v", gray.Bounds())
	}
}

func TestPreprocessResultGrayImageRejectsInvalidData(t *testing.T) {
	if _, err := (&PreprocessResult{Width: 2, Height: 2, Data: []float32{0}}).GrayImage(); err == nil {
		t.Fatal("expected invalid data length error")
	}
}
